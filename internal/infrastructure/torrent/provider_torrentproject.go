package torrent

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchTorrentProject searches TorrentProject
func SearchTorrentProject(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://torrentproject2.com/?t=%s&p=%d&orderby=seeders", query, page)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.106 Safari/537.36")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var results []*domain.SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5)

	doc.Find(".tt div").Each(func(i int, s *goquery.Selection) {
		if i > 1 {
			href := s.Find("span").Eq(0).Find("a").AttrOr("href", "")
			name := strings.TrimSpace(s.Find("span:nth-child(1)").Text())

			if href != "" && name != "" {
				detailURL := "https://torrentproject2.com" + href

				torrent := &domain.SearchResult{
					Name:         name,
					Size:         s.Find("span:nth-child(5)").Text(),
					DateUploaded: strings.TrimSpace(s.Find("span:nth-child(4)").Text()),
					Seeders:      strings.TrimSpace(s.Find("span:nth-child(2)").Text()),
					Leechers:     strings.TrimSpace(s.Find("span:nth-child(3)").Text()),
					URL:          detailURL,
				}

				wg.Add(1)
				go func(t *domain.SearchResult) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if magnet, err := fetchTorrentProjectMagnet(t.URL); err == nil {
						t.Magnet = magnet
					}

					mu.Lock()
					results = append(results, t)
					mu.Unlock()
				}(torrent)
			}
		}
	})

	wg.Wait()
	return results, nil
}

func fetchTorrentProjectMagnet(link string) (string, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.106 Safari/537.36")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	rawMagnet := doc.Find(".usite a").AttrOr("href", "")
	// Ref: magnet.slice(startMagnetIdx) and decodeURIComponent
	if idx := strings.Index(rawMagnet, "magnet"); idx != -1 {
		rawMagnet = rawMagnet[idx:]
	}

	decoded, err := url.QueryUnescape(rawMagnet)
	if err != nil {
		return rawMagnet, nil
	}
	return decoded, nil
}
