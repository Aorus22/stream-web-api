package torrent

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"

	"torrent-stream/internal/domain"
)

// Buffer percentage ahead of playback position (30% of total file)
const BufferAheadPercent = 0.30

// Client wraps anacrolix/torrent for our use case
type Client struct {
	client   *torrent.Client
	torrents map[string]*Wrapper
	mu       sync.RWMutex
	cacheDir string
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
	cfg.DisableWebseeds = true
	cfg.ListenPort = 0
	cfg.DropDuplicatePeerIds = true

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	log.Printf("🚀 Torrent client initialized (cache: %s)", cacheDir)

	return &Client{
		client:   client,
		torrents: make(map[string]*Wrapper),
		cacheDir: cacheDir,
	}, nil
}

// AddMagnet adds a torrent from a magnet link
func (c *Client) AddMagnet(magnetURI string) (*torrent.Torrent, error) {
	t, err := c.client.AddMagnet(magnetURI)
	if err != nil {
		return nil, err
	}

	// Wait for metadata with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	select {
	case <-t.GotInfo():
		log.Printf("✅ Got metadata for: %s", t.Name())
	case <-ctx.Done():
		t.Drop()
		return nil, fmt.Errorf("timeout waiting for metadata")
	}

	// Store in our map
	c.mu.Lock()
	c.torrents[t.InfoHash().HexString()] = &Wrapper{
		Torrent: t,
		AddedAt: time.Now(),
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
	log.Printf("🗑️ Removed torrent: %s", infoHashHex)
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
			InfoHash: infoHashHex,
			Name:     "Fetching metadata...",
			Status:   "metadata",
		}, nil
	}

	stats := t.Stats()
	var totalDownloaded int64 = 0
	var files []domain.File

	for i, file := range t.Files() {
		fileStats := c.calculateFileStats(t, file, i, infoHashHex, port)
		files = append(files, fileStats)
		totalDownloaded += fileStats.Downloaded
	}

	progress := float64(0)
	if t.Info().TotalLength() > 0 {
		progress = float64(totalDownloaded) / float64(t.Info().TotalLength()) * 100
	}

	return &domain.Torrent{
		InfoHash:      infoHashHex,
		Name:          t.Name(),
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
	for i := firstPiece; i <= lastPiece; i++ {
		if t.Piece(i).State().Complete {
			piecesReady++
		}
	}

	piecesTotal := lastPiece - firstPiece + 1
	downloaded := int64(piecesReady) * pieceLength

	progress := float64(0)
	if file.Length() > 0 {
		progress = float64(downloaded) / float64(file.Length()) * 100
		if progress > 100 {
			progress = 100
		}
	}

	return domain.File{
		Index:       index,
		Name:        file.DisplayPath(),
		Length:      file.Length(),
		Progress:    progress,
		Downloaded:  downloaded,
		StreamURL:   fmt.Sprintf("/stream/%s/%d", infoHash, index),
		PieceStart:  firstPiece,
		PieceEnd:    lastPiece,
		PiecesReady: piecesReady,
		PiecesTotal: piecesTotal,
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

// GetFileForStreaming returns a file optimized for streaming
func (c *Client) GetFileForStreaming(infoHashHex string, fileIndex int, startByte, endByte int64) (*torrent.File, error) {
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

	// Prioritize pieces for this range
	pieceLength := t.Info().PieceLength
	fileOffset := file.Offset()

	// Calculate which pieces we need
	startPiece := int((fileOffset + startByte) / pieceLength)

	// Buffer ahead: only request 30% of file
	bufferSize := int64(float64(file.Length()) * BufferAheadPercent)
	bufferEnd := startByte + bufferSize
	if bufferEnd > file.Length() {
		bufferEnd = file.Length()
	}
	endPiece := int((fileOffset + bufferEnd) / pieceLength)

	// Prioritize these pieces
	for i := startPiece; i <= endPiece; i++ {
		piece := t.Piece(i)
		if !piece.State().Complete {
			piece.SetPriority(torrent.PiecePriorityNow)
		}
	}

	return file, nil
}

// GetFileReader returns a reader for a file range
func (c *Client) GetFileReader(infoHashHex string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	file, err := c.GetFileForStreaming(infoHashHex, fileIndex, start, end)
	if err != nil {
		return nil, err
	}

	reader := file.NewReader()
	reader.SetReadahead(5 * 1024 * 1024) // 5MB readahead
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

	// Formats supported natively by browsers
	supportedFormats := map[string]bool{
		"mp4":  true,
		"webm": true,
		"ogg":  true,
		"ogv":  true,
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
}

// Helper for port string
func getPortStr(port int) string {
	if port == 0 {
		return "8080"
	}
	return strconv.Itoa(port)
}
