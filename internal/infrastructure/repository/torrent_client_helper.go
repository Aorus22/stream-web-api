package repository

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"

	"stream-web-api/internal/domain/model"
)

type RawTorrent = torrent.Torrent

func GetFileHandle(t *torrent.Torrent, fileIndex int) *model.FileHandle {
	if t == nil || t.Info() == nil {
		return nil
	}
	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return nil
	}
	file := files[fileIndex]
	return &model.FileHandle{
		InfoHash:    t.InfoHash().HexString(),
		FileIndex:   fileIndex,
		Length:      file.Length(),
		DisplayPath: file.DisplayPath(),
		PieceLength: t.Info().PieceLength,
		Offset:      file.Offset(),
	}
}

func IsTorrentReady(raw interface{}) bool {
	t, ok := raw.(*torrent.Torrent)
	if !ok {
		return false
	}
	return t != nil && t.Info() != nil
}

func GetHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			DisableKeepAlives: true,
		},
	}
}

func PrepareRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://google.com")

	return req, nil
}
