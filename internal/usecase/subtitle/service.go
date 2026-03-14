package subtitle

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// Disable automatic decompression to handle it manually with magic bytes
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

	// Essential headers for OpenSubtitles mirrors
	req.Header.Set("User-Agent", "OpenSubtitlesPlayer v1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.opensubtitles.org/")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	
	var bodyBytes []byte
	var useProxy bool

	if err != nil {
		log.Printf("Download error for %s: %v. Will try proxy.", link, err)
		useProxy = true
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("Download status %d. Will try proxy.", resp.StatusCode)
			useProxy = true
		} else {
			bodyBytes, err = io.ReadAll(resp.Body)
			if err != nil || len(bodyBytes) == 0 {
				log.Printf("Read error or 0 bytes. Will try proxy.")
				useProxy = true
			}
		}
	}

	// Fallback mechanism for ISP blocking (e.g., Internet Baik returning 0 bytes or connection reset)
	if useProxy {
		log.Printf("DEBUG: Attempting download via proxy for: %s", link)
		proxyURL := "https://api.codetabs.com/v1/proxy?quest=" + url.QueryEscape(link)
		req, _ := http.NewRequest("GET", proxyURL, nil)
		req.Header.Set("User-Agent", "OpenSubtitlesPlayer v1")
		
		proxyResp, proxyErr := client.Do(req)
		if proxyErr == nil {
			defer proxyResp.Body.Close()
			if proxyResp.StatusCode == http.StatusOK {
				bodyBytes, _ = io.ReadAll(proxyResp.Body)
				log.Printf("DEBUG: Proxy download successful, bytes: %d", len(bodyBytes))
			} else {
				return nil, fmt.Errorf("proxy download failed with status: %d", proxyResp.StatusCode)
			}
		} else {
			return nil, fmt.Errorf("proxy download error: %v", proxyErr)
		}
	}

	// Handle GZIP
	var srtContent []byte
	// Check for GZIP magic number (0x1f 0x8b)
	if len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("GZIP error (magic detected but failed to read): %v", err)
			// Fallback: maybe it's not actually gzip despite magic bytes?
			srtContent = bodyBytes
		} else {
			defer gr.Close()
			srtContent, err = io.ReadAll(gr)
			if err != nil {
				log.Printf("GZIP ReadAll error: %v", err)
				return nil, err
			}
		}
	} else {
		srtContent = bodyBytes
	}

	// Parse SRT
	log.Printf("DEBUG: Decompressed size: %d, Content preview: %q", len(srtContent), string(srtContent[:min(len(srtContent), 200)]))
	cues := srt.Parse(srtContent)
	log.Printf("DEBUG: Converted SRT to %d cues", len(cues))

	if len(cues) == 0 {
		// If srtContent looks like HTML, OpenSubtitles might have served a landing/error page
		if len(srtContent) > 10 && strings.Contains(strings.ToLower(string(srtContent[:100])), "<html") {
			return nil, fmt.Errorf("received HTML instead of subtitle (blocked or bot detection)")
		}
		return nil, fmt.Errorf("no cues parsed from subtitle file (size: %d bytes)", len(srtContent))
	}

	return cues, nil
}

// DownloadRaw downloads raw subtitle content (for autosync)
func (s *Service) DownloadRaw(link string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OpenSubtitlesPlayer v1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://www.opensubtitles.org/")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)

	var bodyBytes []byte
	var useProxy bool

	if err != nil {
		useProxy = true
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			useProxy = true
		} else {
			bodyBytes, err = io.ReadAll(resp.Body)
			if err != nil || len(bodyBytes) == 0 {
				useProxy = true
			}
		}
	}

	// Fallback mechanism for ISP blocking (e.g., Internet Baik returning 0 bytes or connection reset)
	if useProxy {
		log.Printf("DEBUG: Attempting download via proxy for: %s", link)
		proxyURL := "https://api.codetabs.com/v1/proxy?quest=" + url.QueryEscape(link)
		req, _ := http.NewRequest("GET", proxyURL, nil)
		req.Header.Set("User-Agent", "OpenSubtitlesPlayer v1")
		
		proxyResp, proxyErr := client.Do(req)
		if proxyErr == nil {
			defer proxyResp.Body.Close()
			if proxyResp.StatusCode == http.StatusOK {
				bodyBytes, _ = io.ReadAll(proxyResp.Body)
				log.Printf("DEBUG: Proxy download successful, bytes: %d", len(bodyBytes))
			}
		}
	}

	// Handle GZIP
	if len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return bodyBytes, nil
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}

	return bodyBytes, nil
}
