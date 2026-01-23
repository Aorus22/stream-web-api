package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchBitSearch searches BitSearch
func SearchBitSearch(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://bitsearch.to/search?q=%s&page=%d&sort=seeders", query, page)
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

	doc.Find("li.search-result.view-box").Each(func(i int, s *goquery.Selection) {
		size := s.Find(".info div div").Eq(2).Text()
		if size != "" {
			torrent := &domain.SearchResult{
				Name:         strings.TrimSpace(s.Find(".info h5 a").Text()),
				Size:         strings.TrimSpace(size),
				Downloads:    strings.TrimSpace(s.Find(".info div div").Eq(1).Text()),
				Seeders:      strings.TrimSpace(s.Find(".info div div").Eq(3).Text()),
				Leechers:     strings.TrimSpace(s.Find(".info div div").Eq(4).Text()),
				DateUploaded: strings.TrimSpace(s.Find(".info div div").Eq(5).Text()),
				URL:          "https://bitsearch.to" + s.Find(".info h5 a").AttrOr("href", ""),
				Magnet:       s.Find(".links a").Next().AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
