package torrent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchTorrentGalaxy searches TorrentGalaxy
func SearchTorrentGalaxy(query string, page int) ([]*domain.SearchResult, error) {
	if page > 0 {
		page = page - 1 // Reference code subtracts 1 for paging
	}
	url := fmt.Sprintf("https://torrentgalaxy.to/torrents.php?search=%s&sort=id&order=desc&page=%d", query, page)
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

	doc.Find("div.tgxtablerow.txlight").Each(func(i int, s *goquery.Selection) {
		name := s.Find(":nth-child(4) div a b").Text()
		if name != "" {
			poster := s.AttrOr("onmouseover", "")
			// Regex extraction for poster from onmouseover string
			// Ref: posterRegex = /\bhttps?:[^)''"]+\.(?:jpg|jpeg|gif|png)(?![a-z])/g;
			// We can do simple substring or regex
			// Simple attempt: look for http...jpg/png
			if strings.Contains(poster, "src='") {
				p1 := strings.Index(poster, "src='") + 5
				p2 := strings.Index(poster[p1:], "'")
				if p2 != -1 {
					poster = poster[p1 : p1+p2]
				}
			} else {
				poster = ""
			}

			torrent := &domain.SearchResult{
				Name:         name,
				Poster:       poster,
				Category:     s.Find(":nth-child(1) a small").Text(),
				URL:          "https://torrentgalaxy.to" + s.Find("a.txlight").AttrOr("href", ""),
				UploadedBy:   s.Find(":nth-child(7) span a span").Text(),
				Size:         s.Find(":nth-child(8)").Text(),
				Seeders:      s.Find(":nth-child(11) span font:nth-child(1)").Text(),
				Leechers:     s.Find(":nth-child(11) span font:nth-child(2)").Text(),
				DateUploaded: s.Find(":nth-child(12)").Text(),
				Magnet:       s.Find(".tgxtablecell.collapsehide.rounded.txlight a").Next().AttrOr("href", ""),
			}
			results = append(results, torrent)
		}
	})

	return results, nil
}
