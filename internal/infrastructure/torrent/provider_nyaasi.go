package torrent

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchNyaaSI searches NyaaSI
func SearchNyaaSI(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://nyaa.si/?f=0&c=0_0&q=%s&p=%d", query, page)

	req, err := PrepareRequest(url)
	if err != nil {
		return nil, err
	}

	client := GetHTTPClient()
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
		name := strings.TrimSpace(s.Find("td[colspan='2'] a").Not(".comments").Text())
		if name == "" {
			// Try finding generic 'a' if structure varies
			name = strings.TrimSpace(s.Find("td").Eq(1).Find("a").Not(".comments").Text())
		}

		href := s.Find("td[colspan='2'] a").Not(".comments").AttrOr("href", "")
		if href == "" {
			href = s.Find("td").Eq(1).Find("a").Not(".comments").AttrOr("href", "")
		}

		if name != "" {
			torrent := &domain.SearchResult{
				Name:         name,
				Category:     s.Find("a").AttrOr("title", ""),
				URL:          "https://nyaa.si" + href,
				Size:         s.Find("td").Eq(3).Text(),
				DateUploaded: s.Find("td").Eq(4).Text(),
				Seeders:      s.Find("td").Eq(5).Text(),
				Leechers:     s.Find("td").Eq(6).Text(),
				Downloads:    s.Find("td").Eq(7).Text(),
				Magnet:       s.Find(".text-center a").Next().AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
