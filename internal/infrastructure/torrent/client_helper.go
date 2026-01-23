package torrent

import (
	"crypto/tls"
	"net/http"
	"time"
)

// GetHTTPClient returns a configured HTTP client that mimics a browser
// and ignores specific TLS issues commonly found with scraping sites.
func GetHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Many legacy sites or sites behind simple CDNs might fail with strict modern TLS/Cipher checking in Go
				// However, nyaa.si specifically often requires standard browser-like handshakes.
				// InsecureSkipVerify is generally not recommended but often necessary for scraping tools if cert chains are weird or intercepted.
				// For Nyaa, the issue is often Cloudflare or protocol version.
				MinVersion: tls.VersionTLS12,
			},
			DisableKeepAlives: true,
		},
	}
}

// PrepareRequest creates a request with standard browser headers
func PrepareRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Common browser headers to reduce blocking chances
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://google.com")

	return req, nil
}
