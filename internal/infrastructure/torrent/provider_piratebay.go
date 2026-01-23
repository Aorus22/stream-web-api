package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchPirateBay searches PirateBay
func SearchPirateBay(query string, page int) ([]*domain.SearchResult, error) {
	// Reference uses /99/0 (Video category? or generic?) - preserving exact URL structure from reference
	url := fmt.Sprintf("https://thehiddenbay.com/search/%s/%d/99/0", query, page)
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

	doc.Find("table#searchResult tr").Each(func(i int, s *goquery.Selection) {
		name := s.Find("a.detLink").Text()
		if name != "" {
			desc := s.Find("font.detDesc").Text()
			// Parse desc: "Uploaded 09-09 2011, Size 1.39 GiB, ULed by YIFY"
			// Ref: replace(/(Size|Uploaded)/gi, '').replace(/ULed/gi, 'Uploaded').split(',')

			size := ""
			date := ""
			uploadedBy := ""

			// Simple parsing logic
			parts := strings.Split(desc, ",")
			if len(parts) >= 1 {
				date = strings.TrimSpace(strings.Replace(parts[0], "Uploaded", "", -1))
			}
			if len(parts) >= 2 {
				size = strings.TrimSpace(strings.Replace(parts[1], "Size", "", -1))
			}
			if len(parts) >= 3 {
				uploadedBy = strings.TrimSpace(strings.Replace(parts[2], "ULed by", "", -1))
			}

			// Also extract uploader from link if exists
			uploaderLink := s.Find("font.detDesc a").Text()
			if uploaderLink != "" {
				uploadedBy = uploaderLink
			}

			torrent := &domain.SearchResult{
				Name:         name,
				Size:         size,
				DateUploaded: date,
				Category:     s.Find("td.vertTh center a").Eq(0).Text(),
				Seeders:      s.Find("td").Eq(2).Text(),
				Leechers:     s.Find("td").Eq(3).Text(),
				UploadedBy:   uploadedBy,
				URL:          s.Find("a.detLink").AttrOr("href", ""),
				Magnet:       s.Find("td div.detName").Next().AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
