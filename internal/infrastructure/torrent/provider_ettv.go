package torrent

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchEttvCentral searches EttvCentral
func SearchEttvCentral(query string, page int) ([]*domain.SearchResult, error) {
	// Ref: query... &page=... (JS uses page='0' default, so page might be 0-indexed)
	// We'll stick to passing the int directly.
	url := fmt.Sprintf("https://www.ettvcentral.com/torrents-search.php?search=%s&page=%d", query, page)
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

	doc.Find("table tbody tr").Each(func(i int, s *goquery.Selection) {
		td := s.Find("td")
		name := td.Eq(1).Find("a b").Text()
		href := td.Eq(1).Find("a").AttrOr("href", "")

		if name != "" && href != "" {
			detailURL := "https://www.ettvcentral.com" + href

			torrent := &domain.SearchResult{
				Name:         name,
				Category:     td.Eq(0).Find("a img").AttrOr("title", ""),
				DateUploaded: td.Eq(2).Text(),
				Size:         td.Eq(3).Text(),
				Seeders:      td.Eq(5).Text(),
				Leechers:     td.Eq(6).Text(),
				UploadedBy:   td.Eq(7).Text(),
				URL:          detailURL,
			}

			wg.Add(1)
			go func(t *domain.SearchResult) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				if magnet, poster, err := fetchEttvDetails(t.URL); err == nil {
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

func fetchEttvDetails(url string) (string, string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", "", err
	}

	poster := doc.Find("div .torrent_data center img").AttrOr("src", "")
	magnet := doc.Find("#downloadbox > table > tbody > tr > td:nth-child(1) > a").AttrOr("href", "")

	return magnet, poster, nil
}
