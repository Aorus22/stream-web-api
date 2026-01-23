package torrent

import (
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

// GetFileReader returns a reader for streaming
func (s *Service) GetFileReader(infoHash string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	return s.client.GetFileReader(infoHash, fileIndex, start, end)
}

// GetFileForStreaming prepares a file for streaming
func (s *Service) GetFileForStreaming(infoHash string, fileIndex int, start, end int64) error {
	_, err := s.client.GetFileForStreaming(infoHash, fileIndex, start, end)
	return err
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

// GetPort returns the server port
func (s *Service) GetPort() int {
	return s.port
}
