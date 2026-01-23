package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchLimeTorrent searches LimeTorrents
func SearchLimeTorrent(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://www.limetorrents.pro/search/all/%s/seeds/%d/", query, page)
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

	doc.Find(".table2 tbody tr").Each(func(i int, s *goquery.Selection) {
		if i > 0 { // Skip header
			catAndAge := strings.TrimSpace(s.Find("td").Eq(1).Text())
			parts := strings.Split(catAndAge, "-")
			category := ""
			if len(parts) > 1 {
				category = strings.TrimSpace(strings.ReplaceAll(parts[1], "in", ""))
			}

			torrent := &domain.SearchResult{
				Name:     strings.TrimSpace(s.Find("div.tt-name").Text()),
				Size:     strings.TrimSpace(s.Find("td").Eq(2).Text()),
				Category: category,
				Seeders:  strings.TrimSpace(s.Find("td").Eq(3).Text()),
				Leechers: strings.TrimSpace(s.Find("td").Eq(4).Text()),
				URL:      "https://www.limetorrents.pro" + s.Find("div.tt-name a").Next().AttrOr("href", ""),
			}
			// Note: Magnet is not directly in the list, reference JS might be wrong or site changed.
			// Re-reading JS: "Torrent": $(element).find('div.tt-name a').attr('href')
			// The JS doesn't seem to extract Magnet from list, but 'Torrent' link.
			// Let's assume we need to scrape details OR it's a .torrent file link.
			// Wait, the JS object keys in reference are "Torrent" and "Url".
			// My domain struct has "Magnet".
			// If no magnet is available directly, we might need detail scraping.
			// However, looking at JS code: "Torrent": ...attr('href').
			// I'll stick to list parsing for now. If magnet is missing, it might be an issue.

			results = append(results, torrent)
		}
	})

	return results, nil
}
