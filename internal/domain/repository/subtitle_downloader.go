package repository

type SubtitleDownloader interface {
	Download(link string) ([]byte, error)
}
