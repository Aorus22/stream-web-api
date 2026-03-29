package torrent

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

	"github.com/anacrolix/torrent"

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/persistence"
)

// SeekThreshold is the minimum jump in seconds to be considered a "seek" event.
// When the user jumps more than this from the current position, the client
// triggers pre-generation of upcoming HLS segments.
const SeekThreshold = 30.0

// PrefetchSegments is the number of HLS segments to pre-generate ahead
// of the seek position in a background goroutine.
const PrefetchSegments = 3

// PlaybackState tracks the single user's current playback position.
// Since this is a single-user system, there is exactly one playback state.
type PlaybackState struct {
	InfoHash    string    // active torrent info hash
	FileIndex   int       // active file index within the torrent
	Timestamp   float64   // current playback position in seconds
	Duration    float64   // total duration of the video in seconds (0 if unknown)
	SegmentIdx  int       // last requested HLS segment index
	LastUpdated time.Time // when the state was last updated
}

// SeekCallback is called when a seek event is detected.
// It receives the seek target timestamp and segment index.
type SeekCallback func(infoHash string, fileIndex int, segmentIdx int, timestamp float64)

// Client wraps anacrolix/torrent for our use case.
// Designed as a singleton — only one instance should exist per application.
// Manages all torrent state: active torrents, stream lifecycle, and download sequencing.
type Client struct {
	client   *torrent.Client
	torrents map[string]*Wrapper
	mu       sync.RWMutex
	cacheDir string
	repo     *persistence.TorrentRepository

	// Single-user stream management
	// Only one active user stream at a time. New streams kill the previous one.
	streamMu     sync.Mutex
	activeCancel context.CancelFunc
	activeStream string // description of active stream for logging

	// Playback state tracking (single user)
	playbackMu    sync.Mutex
	playback      PlaybackState
	seekCallbacks []SeekCallback

	// Download state: tracks which pieces are currently requested for download.
	// On seek, we cancel the skipped range and request from the new position to end.
	downloadMu       sync.Mutex
	downloadInfoHash string
	downloadFileIdx  int
	downloadBegin    int // first piece currently being downloaded (inclusive)
	downloadEnd      int // last piece currently being downloaded (exclusive, matches DownloadPieces convention)
}

// Wrapper wraps a torrent with additional metadata
type Wrapper struct {
	Torrent   *torrent.Torrent
	AddedAt   time.Time
	LastRead  time.Time
	ReadBytes int64
}

// NewClient creates a new torrent client
func NewClient(cacheDir string) (*Client, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = cacheDir
	cfg.Seed = false
	cfg.DisableUTP = false
	cfg.DisableTCP = false
	cfg.NoDHT = false
	cfg.DisableWebseeds = false // Enable Webseeds for faster speed (Stremio-like)
	cfg.ListenPort = 0
	cfg.DropDuplicatePeerIds = true

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	repo, err := persistence.NewTorrentRepository(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init persistence: %w", err)
	}

	c := &Client{
		client:   client,
		torrents: make(map[string]*Wrapper),
		cacheDir: cacheDir,
		repo:     repo,
	}

	// Restore active torrents
	activeTorrents, err := repo.List()
	if err != nil {
		log.Printf("⚠️ Failed to load active torrents: %v", err)
	} else {
		log.Printf("📂 Loading %d active torrents from DB...", len(activeTorrents))
		for _, t := range activeTorrents {
			go func(magnet string) {
				// Re-add in background to not block startup
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

// AddMagnet adds a torrent from a magnet link
func (c *Client) AddMagnet(magnetURI string) (*torrent.Torrent, error) {
	t, err := c.client.AddMagnet(magnetURI)
	if err != nil {
		return nil, err
	}

	infoHash := t.InfoHash().HexString()

	// Check if we already have this torrent
	c.mu.RLock()
	_, exists := c.torrents[infoHash]
	c.mu.RUnlock()

	if exists {
		// Already managed. Just wait for info if requested (but don't drop on timeout)
		// We'll give it a shorter timeout since it's already running
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		select {
		case <-t.GotInfo():
			return t, nil
		case <-ctx.Done():
			// Don't drop, just return error or maybe even the torrent without metadata
			return nil, fmt.Errorf("timeout waiting for metadata (existing torrent)")
		}
	}

	// Wait for metadata with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	select {
	case <-t.GotInfo():
		log.Printf("✅ Got metadata for: %s", t.Name())
	case <-ctx.Done():
		// Check map again before dropping to prevent race condition
		c.mu.RLock()
		_, existsNow := c.torrents[infoHash]
		c.mu.RUnlock()

		if !existsNow {
			// Safely drop with panic recovery
			defer func() {
				if r := recover(); r != nil {
					log.Printf("⚠️ Recovered from panic in t.Drop(): %v", r)
				}
			}()
			t.Drop()
		}
		return nil, fmt.Errorf("timeout waiting for metadata")
	}

	// Store in our map
	c.mu.Lock()
	// Double check
	if _, exists := c.torrents[infoHash]; !exists {
		c.torrents[infoHash] = &Wrapper{
			Torrent: t,
			AddedAt: time.Now(),
		}
		// Save to DB
		if err := c.repo.Add(infoHash, magnetURI); err != nil {
			log.Printf("⚠️ Failed to persist torrent %s: %v", infoHash, err)
		}
	}
	c.mu.Unlock()

	return t, nil
}

// GetTorrent returns a torrent by info hash
func (c *Client) GetTorrent(infoHashHex string) *torrent.Torrent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	wrapper, exists := c.torrents[infoHashHex]
	if !exists {
		return nil
	}
	return wrapper.Torrent
}

// GetWrapper returns the wrapper for advanced operations
func (c *Client) GetWrapper(infoHashHex string) *Wrapper {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.torrents[infoHashHex]
}

// RemoveTorrent removes a torrent
func (c *Client) RemoveTorrent(infoHashHex string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wrapper, exists := c.torrents[infoHashHex]
	if !exists {
		return fmt.Errorf("torrent not found: %s", infoHashHex)
	}

	wrapper.Torrent.Drop()
	delete(c.torrents, infoHashHex)

	// Remove from DB
	if err := c.repo.Remove(infoHashHex); err != nil {
		log.Printf("⚠️ Failed to remove torrent from DB %s: %v", infoHashHex, err)
	}

	log.Printf("🗑️ Removed torrent: %s", infoHashHex)
	return nil
}

// RemoveAll removes all torrents
func (c *Client) RemoveAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for hash, wrapper := range c.torrents {
		wrapper.Torrent.Drop()
		delete(c.torrents, hash)
	}

	// Remove from DB
	if err := c.repo.RemoveAll(); err != nil {
		log.Printf("⚠️ Failed to clear torrents from DB: %v", err)
		return err
	}

	log.Printf("🗑️ Removed all torrents")
	return nil
}

// GetStats returns statistics for a torrent as domain entity
func (c *Client) GetStats(infoHashHex string, baseURL string, port int) (*domain.Torrent, error) {
	c.mu.RLock()
	wrapper, exists := c.torrents[infoHashHex]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("torrent not found: %s", infoHashHex)
	}

	t := wrapper.Torrent

	if t.Info() == nil {
		return &domain.Torrent{
			InfoHash:  infoHashHex,
			Name:      "Fetching metadata...",
			MagnetURI: fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHashHex),
			Status:    "metadata",
		}, nil
	}

	stats := t.Stats()
	var files []domain.File

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

	return &domain.Torrent{
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

func (c *Client) calculateFileStats(t *torrent.Torrent, file *torrent.File, index int, infoHash string, port int) domain.File {
	pieceLength := t.Info().PieceLength
	fileOffset := file.Offset()
	fileEnd := fileOffset + file.Length()

	firstPiece := int(fileOffset / pieceLength)
	lastPiece := int((fileEnd - 1) / pieceLength)

	piecesReady := 0
	var bufferedRanges []domain.Range
	var currentRange *domain.Range

	for i := firstPiece; i <= lastPiece; i++ {
		if t.Piece(i).State().Complete {
			piecesReady++

			// Calculate intersection of piece and file
			pieceStart := int64(i) * pieceLength
			pieceEnd := pieceStart + pieceLength

			// Clip to file bounds
			start := pieceStart
			if start < fileOffset {
				start = fileOffset
			}
			end := pieceEnd
			if end > fileEnd {
				end = fileEnd
			}

			// Relative to file start
			relStart := start - fileOffset
			relEnd := end - fileOffset

			if currentRange == nil {
				currentRange = &domain.Range{Start: relStart, End: relEnd}
			} else {
				// Extend current range if adjacent
				if relStart <= currentRange.End {
					if relEnd > currentRange.End {
						currentRange.End = relEnd
					}
				} else {
					bufferedRanges = append(bufferedRanges, *currentRange)
					currentRange = &domain.Range{Start: relStart, End: relEnd}
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

	return domain.File{
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

// ListTorrents returns all active torrents
func (c *Client) ListTorrents(port int) []*domain.Torrent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get all wrappers first to sort them
	var wrappers []*Wrapper
	for _, w := range c.torrents {
		wrappers = append(wrappers, w)
	}

	// Sort by AddedAt descending (newest first)
	sort.Slice(wrappers, func(i, j int) bool {
		return wrappers[i].AddedAt.After(wrappers[j].AddedAt)
	})

	var stats []*domain.Torrent
	for _, w := range wrappers {
		infoHash := w.Torrent.InfoHash().HexString()
		s, err := c.GetStats(infoHash, "", port)
		if err == nil {
			stats = append(stats, s)
		}
	}
	return stats
}

// GetFileReader returns a reader for a file range
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
	reader.SetReadahead(5 * 1024 * 1024) // 5MB Readahead
	reader.SetResponsive()

	if start > 0 {
		reader.Seek(start, io.SeekStart)
	}

	return reader, nil
}

// StartFileDownload begins downloading the entire file sequentially from piece 0 to end.
// This should be called once when the user first accesses a file (e.g. HLS playlist request).
// Anacrolix/torrent will download pieces in order from begin to end.
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

// SeekFileDownload moves the download pointer to a new position.
// It cancels pieces between the old start and the new seek position (the skipped gap),
// then requests download from the seek position to the end of the file.
// Already-downloaded pieces in the cancelled range are kept (CancelPieces only affects pending downloads).
//
// Example: downloading [0, 1000). User seeks to piece 500.
//   - Cancel [current_download_begin, 500) — stop downloading the gap we skipped
//   - DownloadPieces [500, 1000) — download from seek position to end
//   - Result: pieces [0..current_pos] already downloaded + [500..1000) downloading
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

	// Calculate the seek piece from timestamp/duration proportion
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
		// No duration known, can't seek accurately
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
	// Cancel the skipped range: from wherever we were downloading up to the seek point
	// This stops downloading pieces we jumped over
	if oldBegin < seekPiece && c.downloadInfoHash == infoHashHex && c.downloadFileIdx == fileIndex {
		t.CancelPieces(oldBegin, seekPiece)
		log.Printf("⏭️ Cancelled download for skipped pieces [%d, %d)", oldBegin, seekPiece)
	}

	c.downloadInfoHash = infoHashHex
	c.downloadFileIdx = fileIndex
	c.downloadBegin = seekPiece
	c.downloadEnd = fileEnd
	c.downloadMu.Unlock()

	// Request download from seek position to end
	t.DownloadPieces(seekPiece, fileEnd)
	log.Printf("📥 Seek download: pieces [%d, %d) (time: %.1fs, piece %d/%d)",
		seekPiece, fileEnd, timestamp, seekPiece-fileBegin, totalPieces)

	return nil
}

// WaitForPieces waits for specific pieces to be downloaded with timeout
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

// GetPieceInfo returns information about pieces for a file
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

// NeedsTranscoding checks if a file needs transcoding based on extension
func NeedsTranscoding(filename string) bool {
	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx+1:]
	}

	// Formats supported natively by browsers (empty to force transcoding for all)
	supportedFormats := map[string]bool{}

	return !supportedFormats[ext]
}

// GetMimeType returns MIME type for a file
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

// Close closes the client
func (c *Client) Close() {
	c.client.Close()
	if c.repo != nil {
		c.repo.Close()
	}
}

// --- Single-user stream management ---
// These methods enforce that only one user stream is active at a time.
// Internal requests (like FFmpeg loopback) bypass this.

// AcquireStream registers a new active user stream.
// It cancels any previously active stream and returns a context that will be
// cancelled when this stream is superseded or explicitly killed.
// The caller MUST call the returned cancel function when the stream ends.
func (c *Client) AcquireStream(parentCtx context.Context, description string) (context.Context, context.CancelFunc) {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	// Kill previous stream
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

// KillActiveStream cancels the currently active user stream, if any.
// Returns true if a stream was killed, false if there was no active stream.
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

// HasActiveStream returns whether there is currently an active user stream.
func (c *Client) HasActiveStream() bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	return c.activeCancel != nil
}

// ActiveStreamInfo returns the description of the currently active stream.
func (c *Client) ActiveStreamInfo() string {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	return c.activeStream
}

// TorrentCount returns the number of currently managed torrents.
func (c *Client) TorrentCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.torrents)
}

// --- Playback state tracking ---
// Single-user playback position tracking with seek detection.
// When a seek is detected (jump > SeekThreshold), the client automatically
// re-prioritizes torrent pieces around the new position and fires registered callbacks.

// OnSeek registers a callback that fires when a seek event is detected.
// This is used by the handler layer to trigger HLS pre-generation.
func (c *Client) OnSeek(cb SeekCallback) {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()
	c.seekCallbacks = append(c.seekCallbacks, cb)
}

// UpdatePlayback updates the current playback position.
// It detects seek events and automatically:
//   - Re-prioritizes torrent pieces around the new position
//   - Fires registered seek callbacks (for HLS pre-generation)
//
// Returns true if a seek was detected.
func (c *Client) UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()

	prev := c.playback
	seekDetected := false

	// Detect seek: same file but large timestamp jump, or different file entirely
	sameFile := prev.InfoHash == infoHash && prev.FileIndex == fileIndex
	if sameFile {
		delta := timestamp - prev.Timestamp
		// Seek = jump forward >30s OR backward >10s (backward is always intentional)
		if delta > SeekThreshold || delta < -10.0 {
			seekDetected = true
			log.Printf("⏩ Seek detected: %.1fs -> %.1fs (delta: %.1fs)", prev.Timestamp, timestamp, delta)
		}
	} else if prev.InfoHash != "" {
		// Switched to a different file entirely
		seekDetected = true
		log.Printf("⏩ Playback switched: %s/%d -> %s/%d", prev.InfoHash, prev.FileIndex, infoHash, fileIndex)
	}

	// Update state
	c.playback = PlaybackState{
		InfoHash:    infoHash,
		FileIndex:   fileIndex,
		Timestamp:   timestamp,
		Duration:    duration,
		SegmentIdx:  segmentIdx,
		LastUpdated: time.Now(),
	}

	// On seek: move the download pointer to the new position and fire callbacks
	if seekDetected {
		// Move download pointer: cancel skipped pieces, download from seek position to end
		go func() {
			if err := c.SeekFileDownload(infoHash, fileIndex, timestamp, duration); err != nil {
				log.Printf("⚠️ Failed to seek file download: %v", err)
			}
		}()

		// Fire seek callbacks (for HLS pre-generation etc.)
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

// GetPlaybackState returns a snapshot of the current playback state.
func (c *Client) GetPlaybackState() PlaybackState {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()
	return c.playback
}

// UpdatePlaybackDuration sets the video duration after it becomes known (e.g., from ffprobe).
// This is used by SeekFileDownload to map timestamps to piece positions.
func (c *Client) UpdatePlaybackDuration(infoHash string, fileIndex int, duration float64) {
	c.playbackMu.Lock()
	defer c.playbackMu.Unlock()

	if c.playback.InfoHash == infoHash && c.playback.FileIndex == fileIndex {
		c.playback.Duration = duration
		log.Printf("📏 Playback duration updated: %.1fs for %s/%d", duration, infoHash, fileIndex)
	}
}

// SaveMetadata persists metadata JSON for a torrent
func (c *Client) SaveMetadata(infoHash, metadataJSON string) error {
	return c.repo.SaveMetadata(infoHash, metadataJSON)
}

// GetMetadata retrieves metadata JSON for a torrent. Returns ("", nil) if not found.
func (c *Client) GetMetadata(infoHash string) (string, error) {
	return c.repo.GetMetadata(infoHash)
}

// Helper for port string
func getPortStr(port int) string {
	if port == 0 {
		return "8080"
	}
	return strconv.Itoa(port)
}
