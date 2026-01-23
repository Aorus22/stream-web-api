package torrent

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchEzTV searches EzTV
func SearchEzTV(query string, page int) ([]*domain.SearchResult, error) {
	// EzTV doesn't strictly adhere to paging in the same way, but URL is simple
	url := fmt.Sprintf("https://eztv.re/search/%s", query)
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

	// Client-side regex filter from JS: query.replace(/(\W|\s)/ig, '(\\W|\\s|).?')
	// In Go, we'll try a simpler approach or replicate it if needed.
	// The JS code does manual filtering after fetching everything.
	// Let's implement basic filtering if strictly needed, but strict string matching might be enough or we can trust the server search partially.
	// The JS code: if (!name.match((new RegExp(query.replace(/(\W|\s)/ig, '(\\W|\\s|).?'), 'ig')))) return;
	// We'll skip complex regex for now and trust the user/server results, filtering only empties.

	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		link := s.Find("td").Eq(1).Find("a")
		href := link.AttrOr("href", "")
		name := link.Text()

		if href != "" || name != "" {
			torrent := &domain.SearchResult{
				Name:         name,
				Size:         s.Find("td").Eq(3).Text(),
				DateUploaded: s.Find("td").Eq(4).Text(),
				Seeders:      s.Find("td").Eq(5).Text(),
				URL:          "https://eztv.io" + href,
				Magnet:       s.Find("td").Eq(2).Find("a").AttrOr("href", ""),
			}

			// Optional: Re-implement the regex filter if results are poor
			regStr := regexp.MustCompile(`(\W|\s)`).ReplaceAllString(query, `(\W|\s|).?`)
			matched, _ := regexp.MatchString("(?i)"+regStr, name)

			if matched {
				results = append(results, torrent)
			}
		}
	})

	return results, nil
}
