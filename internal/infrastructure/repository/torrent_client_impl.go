package repository

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
	"github.com/anacrolix/torrent"
)

const SeekThreshold = 30.0

const PrefetchSegments = 3

type PlaybackState struct {
	InfoHash    string
	FileIndex   int
	Timestamp   float64
	Duration    float64
	SegmentIdx  int
	LastUpdated time.Time
}

type SeekCallback = domainrepo.SeekCallback

type Client struct {
	client   *torrent.Client
	torrents map[string]*Wrapper
	mu       sync.RWMutex
	cacheDir string
	repo     *TorrentRepository

	streamMu     sync.Mutex
	activeCancel context.CancelFunc
	activeStream string

	playbackMu    sync.Mutex
	playback      PlaybackState
	seekCallbacks []SeekCallback

	downloadMu       sync.Mutex
	downloadInfoHash string
	downloadFileIdx  int
	downloadBegin    int
	downloadEnd      int
}

type Wrapper struct {
	Torrent   *torrent.Torrent
	AddedAt   time.Time
	LastRead  time.Time
	ReadBytes int64
}

func NewClient(cacheDir string) (*Client, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = cacheDir
	cfg.Seed = false
	cfg.DisableUTP = false
	cfg.DisableTCP = false
	cfg.NoDHT = false
	cfg.DisableWebseeds = false
	cfg.ListenPort = 0
	cfg.DropDuplicatePeerIds = true

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	sharedDB, err := NewSharedDB(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init shared DB: %w", err)
	}

	repo := NewTorrentRepository(sharedDB)

	c := &Client{
		client:   client,
		torrents: make(map[string]*Wrapper),
		cacheDir: cacheDir,
		repo:     repo,
	}

	activeTorrents, err := repo.List()
	if err != nil {
		log.Printf("⚠️ Failed to load active torrents: %v", err)
	} else {
		log.Printf("📂 Loading %d active torrents from DB...", len(activeTorrents))
		for _, t := range activeTorrents {
			go func(magnet string) {
				if _, err := c.AddMagnet(magnet); err != nil {
					log.Printf("⚠️ Failed to restore torrent %s: %v", magnet, err)
				}
			}(t.MagnetURI)
		}
	}

	log.Printf("🚀 Torrent client initialized")
	log.Printf("📂 Cache Location: %s (Ensure this has enough space!)", cacheDir)

	return c, nil
}

func (c *Client) AddMagnet(magnetURI string) (string, error) {
	t, err := c.client.AddMagnet(magnetURI)
	if err != nil {
		return "", err
	}

	infoHash := t.InfoHash().HexString()

	c.mu.RLock()
	_, exists := c.torrents[infoHash]
	c.mu.RUnlock()

	if exists {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		select {
		case <-t.GotInfo():
			return infoHash, nil
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for metadata (existing torrent)")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	select {
	case <-t.GotInfo():
		log.Printf("Got metadata for: %s", t.Name())
	case <-ctx.Done():
		c.mu.RLock()
		_, existsNow := c.torrents[infoHash]
		c.mu.RUnlock()

		if !existsNow {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in t.Drop(): %v", r)
				}
			}()
			t.Drop()
		}
		return "", fmt.Errorf("timeout waiting for metadata")
	}

	c.mu.Lock()
	if _, exists := c.torrents[infoHash]; !exists {
		c.torrents[infoHash] = &Wrapper{
			Torrent: t,
			AddedAt: time.Now(),
		}
		if err := c.repo.Add(infoHash, magnetURI); err != nil {
			log.Printf("Failed to persist torrent %s: %v", infoHash, err)
		}
	}
	c.mu.Unlock()

	return infoHash, nil
}

func (c *Client) GetTorrent(infoHashHex string) *torrent.Torrent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	wrapper, exists := c.torrents[infoHashHex]
	if !exists {
		return nil
	}
	return wrapper.Torrent
}

func (c *Client) GetWrapper(infoHashHex string) *Wrapper {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.torrents[infoHashHex]
}

func (c *Client) RemoveTorrent(infoHashHex string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wrapper, exists := c.torrents[infoHashHex]
	if !exists {
		return fmt.Errorf("torrent not found: %s", infoHashHex)
	}

	wrapper.Torrent.Drop()
	delete(c.torrents, infoHashHex)

	if err := c.repo.Remove(infoHashHex); err != nil {
		log.Printf("⚠️ Failed to remove torrent from DB %s: %v", infoHashHex, err)
	}

	log.Printf("🗑️ Removed torrent: %s", infoHashHex)
	return nil
}

func (c *Client) RemoveAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for hash, wrapper := range c.torrents {
		wrapper.Torrent.Drop()
		delete(c.torrents, hash)
	}

	if err := c.repo.RemoveAll(); err != nil {
		log.Printf("⚠️ Failed to clear torrents from DB: %v", err)
		return err
	}

	log.Printf("🗑️ Removed all torrents")
	return nil
}

func (c *Client) GetStats(infoHashHex string, baseURL string, port int) (*model.Torrent, error) {
	c.mu.RLock()
	wrapper, exists := c.torrents[infoHashHex]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("torrent not found: %s", infoHashHex)
	}

	t := wrapper.Torrent

	if t.Info() == nil {
		return &model.Torrent{
			InfoHash:  infoHashHex,
			Name:      "Fetching metadata...",
			MagnetURI: fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHashHex),
			Status:    "metadata",
		}, nil
	}

	stats := t.Stats()
	var files []model.File

	for i, file := range t.Files() {
		fileStats := c.calculateFileStats(t, file, i, infoHashHex, port)
		files = append(files, fileStats)
	}

	totalDownloaded := t.BytesCompleted()

	progress := float64(0)
	if t.Info().TotalLength() > 0 {
		progress = float64(totalDownloaded) / float64(t.Info().TotalLength()) * 100
	}

	magnetURI := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", infoHashHex, url.QueryEscape(t.Name()))

	return &model.Torrent{
		InfoHash:      infoHashHex,
		Name:          t.Name(),
		MagnetURI:     magnetURI,
		TotalLength:   t.Info().TotalLength(),
		Downloaded:    totalDownloaded,
		Progress:      progress,
		Peers:         stats.TotalPeers,
		DownloadSpeed: stats.BytesReadUsefulData.Int64(),
		UploadSpeed:   stats.BytesWrittenData.Int64(),
		AddedAt:       wrapper.AddedAt.Format(time.RFC3339),
		Status:        "downloading",
		Files:         files,
	}, nil
}

func (c *Client) calculateFileStats(t *torrent.Torrent, file *torrent.File, index int, infoHash string, port int) model.File {
	pieceLength := t.Info().PieceLength
	fileOffset := file.Offset()
	fileEnd := fileOffset + file.Length()

	firstPiece := int(fileOffset / pieceLength)
	lastPiece := int((fileEnd - 1) / pieceLength)

	piecesReady := 0
	var bufferedRanges []model.Range
	var currentRange *model.Range

	for i := firstPiece; i <= lastPiece; i++ {
		if t.Piece(i).State().Complete {
			piecesReady++

			pieceStart := int64(i) * pieceLength
			pieceEnd := pieceStart + pieceLength

			start := pieceStart
			if start < fileOffset {
				start = fileOffset
			}
			end := pieceEnd
			if end > fileEnd {
				end = fileEnd
			}

			relStart := start - fileOffset
			relEnd := end - fileOffset

			if currentRange == nil {
				currentRange = &model.Range{Start: relStart, End: relEnd}
			} else {
				if relStart <= currentRange.End {
					if relEnd > currentRange.End {
						currentRange.End = relEnd
					}
				} else {
					bufferedRanges = append(bufferedRanges, *currentRange)
					currentRange = &model.Range{Start: relStart, End: relEnd}
				}
			}
		} else {
			if currentRange != nil {
				bufferedRanges = append(bufferedRanges, *currentRange)
				currentRange = nil
			}
		}
	}
	if currentRange != nil {
		bufferedRanges = append(bufferedRanges, *currentRange)
	}

	piecesTotal := lastPiece - firstPiece + 1
	downloaded := int64(piecesReady) * pieceLength
	if downloaded > file.Length() {
		downloaded = file.Length()
	}

	progress := float64(0)
	if file.Length() > 0 {
		progress = float64(downloaded) / float64(file.Length()) * 100
		if progress > 100 {
			progress = 100
		}
	}

	return model.File{
		Index:          index,
		Name:           file.DisplayPath(),
		Length:         file.Length(),
		Progress:       progress,
		Downloaded:     downloaded,
		StreamURL:      fmt.Sprintf("/stream/%s/%d", infoHash, index),
		PieceStart:     firstPiece,
		PieceEnd:       lastPiece,
		PiecesReady:    piecesReady,
		PiecesTotal:    piecesTotal,
		BufferedRanges: bufferedRanges,
	}
}

func (c *Client) ListTorrents(port int) []*model.Torrent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var wrappers []*Wrapper
	for _, w := range c.torrents {
		wrappers = append(wrappers, w)
	}

	sort.Slice(wrappers, func(i, j int) bool {
		return wrappers[i].AddedAt.After(wrappers[j].AddedAt)
	})

	var stats []*model.Torrent
	for _, w := range wrappers {
		infoHash := w.Torrent.InfoHash().HexString()
		s, err := c.GetStats(infoHash, "", port)
		if err == nil {
			stats = append(stats, s)
		}
	}
	return stats
}

func (c *Client) GetFileReader(infoHashHex string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return nil, fmt.Errorf("torrent not found")
	}
	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return nil, fmt.Errorf("invalid file index")
	}
	file := files[fileIndex]

	reader := file.NewReader()
	reader.SetReadahead(5 * 1024 * 1024)
	reader.SetResponsive()

	if start > 0 {
		reader.Seek(start, io.SeekStart)
	}

	return reader, nil
}

func (c *Client) StartFileDownload(infoHashHex string, fileIndex int) error {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return fmt.Errorf("torrent not found")
	}
	if t.Info() == nil {
		return fmt.Errorf("no metadata")
	}
	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return fmt.Errorf("invalid file index")
	}

	file := files[fileIndex]
	begin := file.BeginPieceIndex()
	end := file.EndPieceIndex()

	c.downloadMu.Lock()
	c.downloadInfoHash = infoHashHex
	c.downloadFileIdx = fileIndex
	c.downloadBegin = begin
	c.downloadEnd = end
	c.downloadMu.Unlock()

	t.DownloadPieces(begin, end)
	log.Printf("📥 Started downloading file %d: pieces [%d, %d) (%d pieces total)",
		fileIndex, begin, end, end-begin)

	return nil
}

func (c *Client) SeekFileDownload(infoHashHex string, fileIndex int, timestamp float64, duration float64) error {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return fmt.Errorf("torrent not found")
	}
	if t.Info() == nil {
		return fmt.Errorf("no metadata")
	}
	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return fmt.Errorf("invalid file index")
	}

	file := files[fileIndex]
	fileBegin := file.BeginPieceIndex()
	fileEnd := file.EndPieceIndex()
	totalPieces := fileEnd - fileBegin

	var seekPiece int
	if duration > 0 {
		proportion := timestamp / duration
		if proportion < 0 {
			proportion = 0
		}
		if proportion > 1 {
			proportion = 1
		}
		seekPiece = fileBegin + int(float64(totalPieces)*proportion)
	} else {
		seekPiece = fileBegin
	}

	if seekPiece >= fileEnd {
		seekPiece = fileEnd - 1
	}
	if seekPiece < fileBegin {
		seekPiece = fileBegin
	}

	c.downloadMu.Lock()
	oldBegin := c.downloadBegin
	if oldBegin < seekPiece && c.downloadInfoHash == infoHashHex && c.downloadFileIdx == fileIndex {
		t.CancelPieces(oldBegin, seekPiece)
		log.Printf("⏭️ Cancelled download for skipped pieces [%d, %d)", oldBegin, seekPiece)
	}

	c.downloadInfoHash = infoHashHex
	c.downloadFileIdx = fileIndex
	c.downloadBegin = seekPiece
	c.downloadEnd = fileEnd
	c.downloadMu.Unlock()

	t.DownloadPieces(seekPiece, fileEnd)
	log.Printf("📥 Seek download: pieces [%d, %d) (time: %.1fs, piece %d/%d)",
		seekPiece, fileEnd, timestamp, seekPiece-fileBegin, totalPieces)

	return nil
}

func (c *Client) WaitForPieces(infoHashHex string, startPiece, endPiece int, timeout time.Duration) error {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return fmt.Errorf("torrent not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pieces")
		case <-ticker.C:
			allReady := true
			for i := startPiece; i <= endPiece; i++ {
				if !t.Piece(i).State().Complete {
					allReady = false
					break
				}
			}
			if allReady {
				return nil
			}
		}
	}
}

func (c *Client) GetPieceInfo(infoHashHex string, fileIndex int) (map[string]interface{}, error) {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return nil, fmt.Errorf("torrent not found")
	}

	if t.Info() == nil {
		return nil, fmt.Errorf("no metadata")
	}

	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return nil, fmt.Errorf("invalid file index")
	}

	file := files[fileIndex]
	pieceLength := t.Info().PieceLength
	fileOffset := file.Offset()
	fileEnd := fileOffset + file.Length()

	firstPiece := int(fileOffset / pieceLength)
	lastPiece := int((fileEnd - 1) / pieceLength)

	var piecesInfo []map[string]interface{}
	for i := firstPiece; i <= lastPiece; i++ {
		state := t.Piece(i).State()
		piecesInfo = append(piecesInfo, map[string]interface{}{
			"index":    i,
			"complete": state.Complete,
			"partial":  state.Partial,
			"checking": state.Checking,
			"priority": int(state.Priority),
		})
	}

	return map[string]interface{}{
		"fileName":    file.DisplayPath(),
		"fileLength":  file.Length(),
		"fileOffset":  fileOffset,
		"pieceLength": pieceLength,
		"firstPiece":  firstPiece,
		"lastPiece":   lastPiece,
		"totalPieces": lastPiece - firstPiece + 1,
		"pieces":      piecesInfo,
	}, nil
}

func NeedsTranscoding(filename string) bool {
	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx+1:]
	}

	supportedFormats := map[string]bool{}

	return !supportedFormats[ext]
}

func GetMimeType(filename string) string {
	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx+1:]
	}

	mimeTypes := map[string]string{
		"mp4":  "video/mp4",
		"webm": "video/webm",
		"mkv":  "video/x-matroska",
		"avi":  "video/x-msvideo",
		"mov":  "video/quicktime",
		"wmv":  "video/x-ms-wmv",
		"flv":  "video/x-flv",
		"ts":   "video/mp2t",
		"m2ts": "video/mp2t",
		"ogv":  "video/ogg",
		"mp3":  "audio/mpeg",
		"flac": "audio/flac",
		"ogg":  "audio/ogg",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func (c *Client) Close() {
	c.client.Close()
}

func (c *Client) AcquireStream(parentCtx context.Context, description string) (context.Context, context.CancelFunc) {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.activeCancel != nil {
		c.activeCancel()
		log.Printf("🔪 Killed previous active stream: %s", c.activeStream)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	c.activeCancel = cancel
	c.activeStream = description

	log.Printf("▶️ New active stream: %s", description)
	return ctx, cancel
}

func (c *Client) KillActiveStream() bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.activeCancel != nil {
		c.activeCancel()
		log.Printf("🔪 Killed active stream: %s", c.activeStream)
		c.activeCancel = nil
		c.activeStream = ""
		return true
	}
	return false
}

func (c *Client) HasActiveStream() bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	return c.activeCancel != nil
}

func (c *Client) ActiveStreamInfo() string {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	return c.activeStream
}

func (c *Client) TorrentCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.torrents)
}

func (c *Client) OnSeek(cb SeekCallback) {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()
	c.seekCallbacks = append(c.seekCallbacks, cb)
}

func (c *Client) UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()

	prev := c.playback
	seekDetected := false

	sameFile := prev.InfoHash == infoHash && prev.FileIndex == fileIndex
	if sameFile {
		delta := timestamp - prev.Timestamp
		if delta > SeekThreshold || delta < -10.0 {
			seekDetected = true
			log.Printf("⏩ Seek detected: %.1fs -> %.1fs (delta: %.1fs)", prev.Timestamp, timestamp, delta)
		}
	} else if prev.InfoHash != "" {
		seekDetected = true
		log.Printf("⏩ Playback switched: %s/%d -> %s/%d", prev.InfoHash, prev.FileIndex, infoHash, fileIndex)
	}

	c.playback = PlaybackState{
		InfoHash:    infoHash,
		FileIndex:   fileIndex,
		Timestamp:   timestamp,
		Duration:    duration,
		SegmentIdx:  segmentIdx,
		LastUpdated: time.Now(),
	}

	if seekDetected {
		go func() {
			if err := c.SeekFileDownload(infoHash, fileIndex, timestamp, duration); err != nil {
				log.Printf("⚠️ Failed to seek file download: %v", err)
			}
		}()

		callbacks := make([]SeekCallback, len(c.seekCallbacks))
		copy(callbacks, c.seekCallbacks)
		go func() {
			for _, cb := range callbacks {
				cb(infoHash, fileIndex, segmentIdx, timestamp)
			}
		}()
	}

	return seekDetected
}

func (c *Client) GetPlaybackState() PlaybackState {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()
	return c.playback
}

func (c *Client) UpdatePlaybackDuration(infoHash string, fileIndex int, duration float64) {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()

	if c.playback.InfoHash == infoHash && c.playback.FileIndex == fileIndex {
		c.playback.Duration = duration
		log.Printf("📏 Playback duration updated: %.1fs for %s/%d", duration, infoHash, fileIndex)
	}
}

func (c *Client) SaveMetadata(infoHash, metadataJSON string) error {
	return c.repo.SaveMetadata(infoHash, metadataJSON)
}

func (c *Client) GetMetadata(infoHash string) (string, error) {
	return c.repo.GetMetadata(infoHash)
}

func (c *Client) GetFileHandle(infoHash string, fileIndex int) (*model.FileHandle, error) {
	t := c.GetTorrent(infoHash)
	if t == nil {
		return nil, fmt.Errorf("torrent not found")
	}
	return GetFileHandle(t, fileIndex), nil
}

func (c *Client) IsTorrentReady(infoHash string) bool {
	t := c.GetTorrent(infoHash)
	if t == nil {
		return false
	}
	return t.Info() != nil
}

func (c *Client) Search(provider, query string, page int) ([]*model.SearchResult, error) {
	return Search(provider, query, page)
}

func (c *Client) GetProviders() []string {
	return GetProviders()
}

func (c *Client) NeedsTranscoding(filename string) bool {
	return NeedsTranscoding(filename)
}

func (c *Client) GetMimeType(filename string) string {
	return GetMimeType(filename)
}

func getPortStr(port int) string {
	if port == 0 {
		return "8080"
	}
	return strconv.Itoa(port)
}
