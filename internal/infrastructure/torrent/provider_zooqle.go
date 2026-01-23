package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchZooqle searches Zooqle
func SearchZooqle(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://zooqle.com/search?pg=%d&q=%s", page, query)
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

	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("td").Eq(1).Find("a").Text())
		if name != "" {
			seedersLeechers := s.Find("td").Eq(5).Find("div").AttrOr("title", "")
			parts := strings.Split(seedersLeechers, "|")
			seeders := ""
			leechers := ""
			if len(parts) >= 1 {
				seeders = strings.TrimSpace(strings.Replace(parts[0], "Seeders:", "", -1))
			}
			if len(parts) >= 2 {
				leechers = strings.TrimSpace(strings.Replace(parts[1], "Leechers:", "", -1))
			}

			torrent := &domain.SearchResult{
				Name:         name,
				Size:         strings.TrimSpace(s.Find("td").Eq(3).Find("div div").Text()),
				DateUploaded: strings.TrimSpace(s.Find("td").Eq(4).Text()),
				Seeders:      seeders,
				Leechers:     leechers,
				URL:          "https://zooqle.com" + s.Find("td").Eq(1).Find("a").AttrOr("href", ""),
				Magnet:       s.Find("td").Eq(2).Find("ul li").Eq(1).Find("a").AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
