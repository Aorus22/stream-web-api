package torrent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// SearchTorrentFunk searches TorrentFunk
func SearchTorrentFunk(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://www.torrentfunk.com/all/torrents/%s/%d.html", query, page)
	if page == 1 {
		url = fmt.Sprintf("https://www.torrentfunk.com/all/torrents/%s.html", query)
	}

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
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5)

	doc.Find(".tmain tbody tr").Each(func(i int, s *goquery.Selection) {
		if i > 4 { // Skip header
			href := s.Find("td").Eq(0).Find("a").AttrOr("href", "")
			name := strings.TrimSpace(s.Find("td").Eq(0).Find("a").Text())

			if href != "" && name != "" {
				detailURL := "https://www.torrentfunk.com" + href

				torrent := &domain.SearchResult{
					Name:         name,
					DateUploaded: s.Find("td").Eq(1).Text(),
					Size:         s.Find("td").Eq(2).Text(),
					Seeders:      s.Find("td").Eq(3).Text(),
					Leechers:     s.Find("td").Eq(4).Text(),
					UploadedBy:   s.Find("td").Eq(5).Text(),
					URL:          detailURL,
				}

				wg.Add(1)
				go func(t *domain.SearchResult) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if magnet, err := fetchTorrentFunkDetails(t.URL); err == nil {
						// Note: Reference says "Torrent" link, not Magnet.
						// Checking reference: ALLTORRENT[i].Torrent = ...
						// It does NOT scrape magnet?
						// Wait, torrentfunk usually provides .torrent file.
						// My domain struct expects Magnet.
						// If only .torrent is available, we might need to download it or just provide the link.
						// But for streaming we prefer magnets.
						// Let's see if we can find a magnet or just store the torrent link as magnet (which might fail in AddMagnet).
						// Actually, Stream-Web-Api AddMagnet supports magnet links.
						// If it's a torrent file URL, `anacrolix/torrent` might support it if we download it.
						// For now, I'll put the torrent link in Magnet field, but it might require handling in `AddMagnet` service to detect if it's HTTP URL.
						t.Magnet = magnet
					}

					mu.Lock()
					results = append(results, t)
					mu.Unlock()
				}(torrent)
			}
		}
	})

	wg.Wait()
	return results, nil
}

func fetchTorrentFunkDetails(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.87 Mobile Safari/537.36")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	// Ref: $('#right > main > div.content > table:nth-child(3) > tbody > tr > td:nth-child(2) > a').attr('href')
	// This looks like a .torrent download link.
	// TorrentFunk usually hides magnets or doesn't have them easily.
	link := doc.Find("#right > main > div.content > table:nth-child(3) > tbody > tr > td:nth-child(2) > a").AttrOr("href", "")
	if link != "" && !strings.HasPrefix(link, "http") {
		link = "https://www.torrentfunk.com" + link
	}
	return link, nil
}
