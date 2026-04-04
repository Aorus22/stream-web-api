package repository

type ActiveTorrentRecord struct {
	InfoHash  string
	MagnetURI string
	AddedAt   string
}

type TorrentRepository interface {
	Add(infoHash, magnetURI string) error
	Remove(infoHash string) error
	RemoveAll() error
	List() ([]ActiveTorrentRecord, error)
	SaveMetadata(infoHash, metadataJSON string) error
	GetMetadata(infoHash string) (string, error)
}
