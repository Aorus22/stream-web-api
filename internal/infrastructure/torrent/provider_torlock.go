package torrent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchTorLock searches TorLock
func SearchTorLock(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://www.torlock.com/all/torrents/%s/%d.html", query, page)
	res, err := http.Get(url)
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

	doc.Find(".table tbody tr").Each(func(i int, s *goquery.Selection) {
		if i > 3 { // Skip header
			href := s.Find("td").Eq(0).Find("div a").AttrOr("href", "")
			name := strings.TrimSpace(s.Find("td").Eq(0).Find("div a b").Text())

			if href != "" && name != "" {
				detailURL := "https://www.torlock.com" + href

				torrent := &domain.SearchResult{
					Name:         name,
					Size:         strings.TrimSpace(s.Find("td").Eq(2).Text()),
					DateUploaded: strings.TrimSpace(s.Find("td").Eq(1).Text()),
					Seeders:      strings.TrimSpace(s.Find("td").Eq(3).Text()),
					Leechers:     strings.TrimSpace(s.Find("td").Eq(4).Text()),
					URL:          detailURL,
				}

				wg.Add(1)
				go func(t *domain.SearchResult) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if magnet, err := fetchTorLockMagnet(t.URL); err == nil {
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

func fetchTorLockMagnet(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
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

	// Ref: $('body > article > table:nth-child(5) > thead > tr > th > div:nth-child(2) > h4 > a:nth-child(1)').attr('href')
	// Simplified selector: look for magnet link generally or refine if needed
	magnet := doc.Find("a[href^='magnet:?']").First().AttrOr("href", "")
	return magnet, nil
}
