package torrent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchKickAss searches KickAssTorrents
func SearchKickAss(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://kickasstorrents.to/usearch/%s/%d/", query, page)
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
	sem := make(chan struct{}, 5) // Semaphore for concurrency limit

	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		if i > 2 { // Skip header rows
			link := s.Find("a.cellMainLink")
			href := link.AttrOr("href", "")
			if href != "" && !strings.HasSuffix(href, "undefined") {
				detailURL := "https://kickasstorrents.to" + href

				torrent := &domain.SearchResult{
					Name:       strings.TrimSpace(link.Text()),
					Size:       strings.TrimSpace(s.Find("td").Eq(1).Text()),
					UploadedBy: strings.TrimSpace(s.Find("td").Eq(2).Text()),
					// Age: s.Find("td").Eq(3).Text(), // Not in struct
					Seeders:  strings.TrimSpace(s.Find("td").Eq(4).Text()),
					Leechers: strings.TrimSpace(s.Find("td").Eq(5).Text()),
					URL:      detailURL,
				}

				wg.Add(1)
				go func(t *domain.SearchResult) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if magnet, err := fetchKickAssMagnet(t.URL); err == nil {
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

func fetchKickAssMagnet(url string) (string, error) {
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

	return doc.Find("a.kaGiantButton").AttrOr("href", ""), nil
}
