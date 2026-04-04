package repository

import (
	"context"
	"io"
	"time"

	"stream-web-api/internal/domain/model"
)

type SeekCallback func(infoHash string, fileIndex int, segmentIdx int, timestamp float64)

type TorrentClient interface {
	AddMagnet(magnetURI string) (string, error)
	GetStats(infoHash string, baseURL string, port int) (*model.Torrent, error)
	ListTorrents(port int) []*model.Torrent
	RemoveTorrent(infoHash string) error
	RemoveAll() error
	GetFileReader(infoHash string, fileIndex int, start, end int64) (io.ReadSeeker, error)
	WaitForPieces(infoHash string, startPiece, endPiece int, timeout time.Duration) error
	GetPieceInfo(infoHash string, fileIndex int) (map[string]interface{}, error)
	GetFileHandle(infoHash string, fileIndex int) (*model.FileHandle, error)
	IsTorrentReady(infoHash string) bool
	EnsureFileHeader(infoHash string, fileIndex int) error
	StartFileDownload(infoHash string, fileIndex int) error
	SeekFileDownload(infoHash string, fileIndex int, timestamp float64, duration float64) error
	AcquireStream(parentCtx context.Context, description string) (context.Context, context.CancelFunc)
	KillActiveStream() bool
	HasActiveStream() bool
	TorrentCount() int
	UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool
	UpdatePlaybackDuration(infoHash string, fileIndex int, duration float64)
	OnSeek(cb SeekCallback)
	SaveMetadata(infoHash, metadataJSON string) error
	GetMetadata(infoHash string) (string, error)
	Search(provider, query string, page int) ([]*model.SearchResult, error)
	GetProviders() []string
	NeedsTranscoding(filename string) bool
	GetMimeType(filename string) string
	Close()
}
