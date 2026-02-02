package torrent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/persistence"
)

// Buffer percentage ahead of playback position (30% of total file)
const BufferAheadPercent = 0.30

// Client wraps anacrolix/torrent for our use case
type Client struct {
	client   *torrent.Client
	torrents map[string]*Wrapper
	mu       sync.RWMutex
	cacheDir string
	repo     *persistence.TorrentRepository
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

	var stats []*domain.Torrent
	for hash := range c.torrents {
		s, err := c.GetStats(hash, "", port)
		if err == nil {
			stats = append(stats, s)
		}
	}
	return stats
}

// UpdatePriorityWindow updates the download window based on playback position
// It implements a "Sliding Window" strategy:
// - Back Buffer (50MB): High (Keep for rewind)
// - Critical (10MB): Now (Play)
// - Forward Buffer (30%): High (Pre-load)
// - Outside: None (Stop download)
func (c *Client) UpdatePriorityWindow(infoHashHex string, fileIndex int, startByte int64) error {
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
	pieceLength := t.Info().PieceLength
	fileOffset := file.Offset()

	// 1. Calculate Windows
	const HeaderSize = 10 * 1024 * 1024                       // 10MB Header
	const BackBufferSize = 50 * 1024 * 1024                   // 50MB Back
	const CriticalSize = 10 * 1024 * 1024                     // 10MB Critical
	forwardBufferSize := int64(float64(file.Length()) * 0.30) // 30% Forward

	// Boundaries (Bytes)
	headerEnd := fileOffset + HeaderSize

	backBufferStart := startByte - BackBufferSize
	if backBufferStart < 0 {
		backBufferStart = 0
	}

	criticalEnd := startByte + CriticalSize
	if criticalEnd > file.Length() {
		criticalEnd = file.Length()
	}

	forwardEnd := startByte + forwardBufferSize
	if forwardEnd > file.Length() {
		forwardEnd = file.Length()
	}

	// Footer
	const FooterSize = 2 * 1024 * 1024
	footerStart := file.Length() - FooterSize
	if footerStart < 0 {
		footerStart = 0
	}

	// Boundaries (Pieces)
	headerEndPiece := int(headerEnd / pieceLength)

	backBufferStartPiece := int((fileOffset + backBufferStart) / pieceLength)
	// Current play position piece
	startPiece := int((fileOffset + startByte) / pieceLength)

	criticalEndPiece := int((fileOffset + criticalEnd) / pieceLength)
	forwardEndPiece := int((fileOffset + forwardEnd) / pieceLength)

	footerStartPiece := int((fileOffset + footerStart) / pieceLength)
	fileEndPiece := int((fileOffset + file.Length() - 1) / pieceLength)

	// 2. Apply Priorities (Strict Loop)
	// We loop through ALL pieces allocated to this file to enforce the window
	// Note: Ideally we loop only file pieces, assuming standard torrent structure

	fileStartPiece := int(fileOffset / pieceLength)
	// fileEndPiece is already calculated

	for i := fileStartPiece; i <= fileEndPiece; i++ {
		// Rule 1: Always Protect Header & Footer
		if i <= headerEndPiece || i >= footerStartPiece {
			if !t.Piece(i).State().Complete {
				t.Piece(i).SetPriority(torrent.PiecePriorityHigh)
			}
			continue
		}

		// Rule 2: Critical Window (Now)
		if i >= startPiece && i <= criticalEndPiece {
			if !t.Piece(i).State().Complete {
				t.Piece(i).SetPriority(torrent.PiecePriorityNow)
			}
			continue
		}

		// Rule 3: Buffer Window (High) - Both Back & Forward
		// Back Buffer: [backBufferStartPiece ... startPiece-1]
		// Forward Buffer: [criticalEndPiece+1 ... forwardEndPiece]
		isBackBuffer := i >= backBufferStartPiece && i < startPiece
		isForwardBuffer := i > criticalEndPiece && i <= forwardEndPiece

		if isBackBuffer || isForwardBuffer {
			if !t.Piece(i).State().Complete {
				t.Piece(i).SetPriority(torrent.PiecePriorityHigh)
			}
			continue
		}

		// Rule 4: Outside Window (None)
		// Deep Past (< backBufferStartPiece) OR Far Future (> forwardEndPiece)
		// We strictly turn these OFF to save bandwidth
		t.Piece(i).SetPriority(torrent.PiecePriorityNone)
	}

	return nil
}

// GetFileReader returns a reader for a file range
func (c *Client) GetFileReader(infoHashHex string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	// Note: We don't call UpdatePriorityWindow here implicitly anymore.
	// It must be called explicitly by the handler.

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
	reader.SetReadahead(1 * 1024 * 1024) // 1MB Readahead
	reader.SetResponsive()

	if start > 0 {
		reader.Seek(start, io.SeekStart)
	}

	return reader, nil
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
	supportedFormats := map[string]bool{
	}

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

// Helper for port string
func getPortStr(port int) string {
	if port == 0 {
		return "8080"
	}
	return strconv.Itoa(port)
}
