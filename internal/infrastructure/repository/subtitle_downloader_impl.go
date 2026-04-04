package repository

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type SubtitleDownloaderImpl struct{}

func NewSubtitleDownloader() *SubtitleDownloaderImpl {
	return &SubtitleDownloaderImpl{}
}

func (d *SubtitleDownloaderImpl) Download(link string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			DisableCompression:  true,
		},
	}

	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

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

	if len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("GZIP error (magic detected but failed to read): %v", err)
			return bodyBytes, nil
		}
		defer gr.Close()
		decompressed, err := io.ReadAll(gr)
		if err != nil {
			log.Printf("GZIP ReadAll error: %v", err)
			return nil, err
		}
		return decompressed, nil
	}

	return bodyBytes, nil
}
