package torrent

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchYTS searches YTS
func SearchYTS(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://yts.mx/browse-movies/%s/all/all/0/latest/0/all?page=%d", query, page)
	if page == 1 {
		url = fmt.Sprintf("https://yts.mx/browse-movies/%s/all/all/0/latest/0/all", query)
	}

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

	doc.Find("div.browse-movie-bottom").Each(func(i int, s *goquery.Selection) {
		href := s.Find("a").AttrOr("href", "")
		if href != "" {
			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// YTS returns multiple torrents (Qualities) for one movie.
				// We will append them all as separate results.
				items, err := fetchYTSDetails(u)
				if err == nil && len(items) > 0 {
					mu.Lock()
					results = append(results, items...)
					mu.Unlock()
				}
			}(href)
		}
	})

	wg.Wait()
	return results, nil
}

func fetchYTSDetails(url string) ([]*domain.SearchResult, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var items []*domain.SearchResult

	baseName := doc.Find("div.hidden-xs h1").Text()
	year := doc.Find("div.hidden-xs h2").Eq(0).Text()
	// genre := doc.Find("div.hidden-xs h2").Eq(1).Text()
	poster := doc.Find("div #movie-poster img").Eq(0).AttrOr("src", "")

	doc.Find("div.modal-torrent").Each(func(i int, s *goquery.Selection) {
		quality := s.Find(":nth-child(1) > span").Text()
		videoType := s.Find(":nth-child(2)").Text()
		size := s.Find(":nth-child(5)").Text()
		magnet := s.Find(":nth-child(7)").AttrOr("href", "")

		// Construct a descriptive name
		fullName := fmt.Sprintf("%s (%s) [%s] %s", baseName, year, quality, videoType)

		item := &domain.SearchResult{
			Name:       fullName,
			Poster:     poster,
			Category:   "Movie",
			UploadedBy: "YTS",
			Size:       size,
			Magnet:     magnet,
			URL:        url,
			Seeders:    "N/A", // Not easily available on detail page without more parsing
			Leechers:   "N/A",
		}
		items = append(items, item)
	})

	return items, nil
}
