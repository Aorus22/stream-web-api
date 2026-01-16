package subtitle

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/opensubtitles"
	"torrent-stream/pkg/srt"
)

// Service provides subtitle business logic
type Service struct {
	osClient *opensubtitles.Client
}

// NewService creates a new subtitle service
func NewService(osClient *opensubtitles.Client) *Service {
	return &Service{
		osClient: osClient,
	}
}

// Search searches for subtitles
func (s *Service) Search(query, lang string) ([]domain.Subtitle, error) {
	return s.osClient.Search(query, lang)
}

// Download downloads and parses subtitle cues from a link
func (s *Service) Download(link string) ([]domain.SubtitleCue, error) {
	resp, err := http.Get(link)
	if err != nil {
		log.Printf("Download error for %s: %v", link, err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle GZIP
	var srtContent []byte
	if len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("GZIP error: %v", err)
			return nil, err
		}
		defer gr.Close()
		srtContent, err = io.ReadAll(gr)
		if err != nil {
			return nil, err
		}
	} else {
		srtContent = bodyBytes
	}

	// Parse SRT
	cues := srt.Parse(srtContent)
	log.Printf("DEBUG: Converted SRT to %d cues", len(cues))

	return cues, nil
}

// DownloadRaw downloads raw subtitle content (for autosync)
func (s *Service) DownloadRaw(link string) ([]byte, error) {
	resp, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle GZIP
	if len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}

	return bodyBytes, nil
}
