package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchGloTorrents searches GloTorrents
func SearchGloTorrents(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://glodls.to/search_results.php?search=%s&sort=seeders&order=desc&page=%d", query, page)
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

	doc.Find(".ttable_headinner tr").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("td").Eq(1).Find("a").Text())
		if name != "" {
			torrent := &domain.SearchResult{
				Name:       name,
				Size:       s.Find("td").Eq(4).Text(),
				UploadedBy: s.Find("td").Eq(7).Find("a b font").Text(),
				Seeders:    s.Find("td").Eq(5).Find("font b").Text(),
				Leechers:   s.Find("td").Eq(6).Find("font b").Text(),
				URL:        "https://glodls.to" + s.Find("td").Eq(1).Find("a").Next().AttrOr("href", ""),
				Magnet:     s.Find("td").Eq(3).Find("a").AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
