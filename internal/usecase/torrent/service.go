package torrent

import (
	"context"
	"io"
	"time"

	"torrent-stream/internal/domain"
	infra "torrent-stream/internal/infrastructure/torrent"
)

// Service provides torrent business logic
type Service struct {
	client *infra.Client
	port   int
}

// NewService creates a new torrent service
func NewService(client *infra.Client, port int) *Service {
	return &Service{
		client: client,
		port:   port,
	}
}

// AddMagnet adds a torrent from magnet URI
func (s *Service) AddMagnet(magnetURI string) (*domain.Torrent, error) {
	t, err := s.client.AddMagnet(magnetURI)
	if err != nil {
		return nil, err
	}

	return s.client.GetStats(t.InfoHash().HexString(), "", s.port)
}

// GetStats returns torrent statistics
func (s *Service) GetStats(infoHash string) (*domain.Torrent, error) {
	return s.client.GetStats(infoHash, "", s.port)
}

// ListTorrents returns all torrents
func (s *Service) ListTorrents() []*domain.Torrent {
	return s.client.ListTorrents(s.port)
}

// RemoveTorrent removes a torrent
func (s *Service) RemoveTorrent(infoHash string) error {
	return s.client.RemoveTorrent(infoHash)
}

// RemoveAllTorrents removes all torrents
func (s *Service) RemoveAllTorrents() error {
	return s.client.RemoveAll()
}

// GetFileReader returns a reader for streaming
func (s *Service) GetFileReader(infoHash string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	return s.client.GetFileReader(infoHash, fileIndex, start, end)
}

// WaitForPieces waits for specific pieces to be ready
func (s *Service) WaitForPieces(infoHash string, startPiece, endPiece int, timeout time.Duration) error {
	return s.client.WaitForPieces(infoHash, startPiece, endPiece, timeout)
}

// GetPieceInfo returns piece information
func (s *Service) GetPieceInfo(infoHash string, fileIndex int) (map[string]interface{}, error) {
	return s.client.GetPieceInfo(infoHash, fileIndex)
}

// GetTorrent returns raw torrent (for internal use)
func (s *Service) GetTorrent(infoHash string) interface{} {
	return s.client.GetTorrent(infoHash)
}

// NeedsTranscoding checks if file needs transcoding
func (s *Service) NeedsTranscoding(filename string) bool {
	return infra.NeedsTranscoding(filename)
}

// GetMimeType returns MIME type for file
func (s *Service) GetMimeType(filename string) string {
	return infra.GetMimeType(filename)
}

// EnsureFileHeader prioritizes file header
func (s *Service) EnsureFileHeader(infoHash string, fileIndex int) error {
	return s.client.EnsureFileHeader(infoHash, fileIndex)
}

// StartFileDownload begins downloading the entire file from piece 0 to end.
func (s *Service) StartFileDownload(infoHash string, fileIndex int) error {
	return s.client.StartFileDownload(infoHash, fileIndex)
}

// SeekFileDownload moves the download pointer to a new position based on timestamp.
func (s *Service) SeekFileDownload(infoHash string, fileIndex int, timestamp float64, duration float64) error {
	return s.client.SeekFileDownload(infoHash, fileIndex, timestamp, duration)
}

// GetPort returns the server port
func (s *Service) GetPort() int {
	return s.port
}

// SearchTorrents searches for torrents
func (s *Service) SearchTorrents(provider, query string, page int) ([]*domain.SearchResult, error) {
	return infra.Search(provider, query, page)
}

// ProviderInfo represents provider information
type ProviderInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	PageType string `json:"pageType"`
}

// SaveMetadata persists metadata JSON for a torrent
func (s *Service) SaveMetadata(infoHash, metadataJSON string) error {
	return s.client.SaveMetadata(infoHash, metadataJSON)
}

// GetMetadata retrieves metadata JSON for a torrent
func (s *Service) GetMetadata(infoHash string) (string, error) {
	return s.client.GetMetadata(infoHash)
}

// GetHardcodedProviders returns hardcoded providers
func (s *Service) GetHardcodedProviders() []string {
	return infra.GetProviders()
}

// --- Stream management (delegated to client) ---

// AcquireStream registers a new active user stream, killing any previous one.
// Returns a cancellable context and cancel function. Caller MUST call cancel when done.
func (s *Service) AcquireStream(parentCtx context.Context, description string) (context.Context, context.CancelFunc) {
	return s.client.AcquireStream(parentCtx, description)
}

// KillActiveStream cancels the active user stream. Returns true if one was killed.
func (s *Service) KillActiveStream() bool {
	return s.client.KillActiveStream()
}

// HasActiveStream returns whether there is an active user stream.
func (s *Service) HasActiveStream() bool {
	return s.client.HasActiveStream()
}

// TorrentCount returns the number of managed torrents.
func (s *Service) TorrentCount() int {
	return s.client.TorrentCount()
}

// --- Playback state tracking (delegated to client) ---

// UpdatePlayback updates the current playback position.
// Detects seeks and auto-triggers piece re-prioritization + registered callbacks.
// Returns true if a seek was detected.
func (s *Service) UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool {
	return s.client.UpdatePlayback(infoHash, fileIndex, timestamp, duration, segmentIdx)
}

// GetPlaybackState returns a snapshot of the current playback state.
func (s *Service) GetPlaybackState() infra.PlaybackState {
	return s.client.GetPlaybackState()
}

// UpdatePlaybackDuration sets the video duration after it becomes known.
func (s *Service) UpdatePlaybackDuration(infoHash string, fileIndex int, duration float64) {
	s.client.UpdatePlaybackDuration(infoHash, fileIndex, duration)
}

// OnSeek registers a callback that fires when a seek event is detected.
func (s *Service) OnSeek(cb infra.SeekCallback) {
	s.client.OnSeek(cb)
}
