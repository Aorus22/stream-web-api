package torrent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchRarbg searches Rarbg
func SearchRarbg(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://rargb.to/search/%d/?search=%s", page, query)
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

	doc.Find("table.lista2t tbody tr.lista2").Each(func(i int, s *goquery.Selection) {
		td := s.Find("td")
		href := td.Eq(1).Find("a").AttrOr("href", "")
		name := td.Eq(1).Find("a").AttrOr("title", "")

		if href != "" {
			detailURL := "https://rargb.to" + href

			torrent := &domain.SearchResult{
				Name:         name,
				Category:     td.Eq(2).Find("a").Text(),
				DateUploaded: td.Eq(3).Text(),
				Size:         td.Eq(4).Text(),
				Seeders:      td.Eq(5).Find("font").Text(),
				Leechers:     td.Eq(6).Text(),
				UploadedBy:   td.Eq(7).Text(),
				URL:          detailURL,
			}

			wg.Add(1)
			go func(t *domain.SearchResult) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				if magnet, poster, err := fetchRarbgDetails(t.URL); err == nil {
					t.Magnet = magnet
					t.Poster = poster
				}

				mu.Lock()
				results = append(results, t)
				mu.Unlock()
			}(torrent)
		}
	})

	wg.Wait()
	return results, nil
}

func fetchRarbgDetails(url string) (string, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.106 Safari/537.36")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", "", err
	}

	poster := "https://rargb.to" + doc.Find("tr:nth-child(4) > td:nth-child(2) > img:nth-child(1)").AttrOr("src", "")
	magnet := doc.Find("tr:nth-child(1) > td:nth-child(2) > a:nth-child(3)").AttrOr("href", "")

	if strings.HasSuffix(poster, "undefined") {
		poster = ""
	}

	return magnet, poster, nil
}
