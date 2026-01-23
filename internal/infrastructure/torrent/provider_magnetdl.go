package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchMagnetDL searches MagnetDL
func SearchMagnetDL(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://magnetdl.abcproxy.org/search/?q=%s&m=1", query)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.87 Mobile Safari/537.36")

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

	doc.Find(".download tbody tr").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("td").Eq(1).Find("a").Text())
		if name != "" {
			torrent := &domain.SearchResult{
				Name:         name,
				Size:         s.Find("td").Eq(5).Text(),
				DateUploaded: s.Find("td").Eq(2).Text(),
				Category:     s.Find("td").Eq(3).Text(),
				Seeders:      s.Find("td").Eq(6).Text(),
				Leechers:     s.Find("td").Eq(7).Text(),
				URL:          "https://www.magnetdl.com" + s.Find("td").Eq(1).Find("a").AttrOr("href", ""),
				Magnet:       s.Find("td").Eq(0).Find("a").AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
