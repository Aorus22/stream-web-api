package repository

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"stream-web-api/internal/domain/model"
)

func SearchTorrentFunk(query string, page int) ([]*model.SearchResult, error) {
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

	var results []*model.SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5)

	doc.Find(".tmain tbody tr").Each(func(i int, s *goquery.Selection) {
		if i > 4 {
			href := s.Find("td").Eq(0).Find("a").AttrOr("href", "")
			name := strings.TrimSpace(s.Find("td").Eq(0).Find("a").Text())

			if href != "" && name != "" {
				detailURL := "https://www.torrentfunk.com" + href

				torrent := &model.SearchResult{
					Name:         name,
					DateUploaded: s.Find("td").Eq(1).Text(),
					Size:         s.Find("td").Eq(2).Text(),
					Seeders:      s.Find("td").Eq(3).Text(),
					Leechers:     s.Find("td").Eq(4).Text(),
					UploadedBy:   s.Find("td").Eq(5).Text(),
					URL:          detailURL,
				}

				wg.Add(1)
				go func(t *model.SearchResult) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if magnet, err := fetchTorrentFunkDetails(t.URL); err == nil {
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

	link := doc.Find("#right > main > div.content > table:nth-child(3) > tbody > tr > td:nth-child(2) > a").AttrOr("href", "")
	if link != "" && !strings.HasPrefix(link, "http") {
		link = "https://www.torrentfunk.com" + link
	}
	return link, nil
}
