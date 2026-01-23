package torrent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"torrent-stream/internal/domain"
)

// Search1337x searches 1337x for torrents
func Search1337x(query string, page int) ([]*domain.SearchResult, error) {
	url := fmt.Sprintf("https://1337xx.to/search/%s/%d/", query, page)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var links []string
	doc.Find("td.name").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Find("a").Next().Attr("href")
		if exists {
			links = append(links, "https://1337xx.to"+href)
		}
	})

	var results []*domain.SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Limit concurrency to avoid blocking/rate limiting
	sem := make(chan struct{}, 5)

	for _, link := range links {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			data, err := scrape1337xDetails(url)
			if err == nil && data != nil {
				mu.Lock()
				results = append(results, data)
				mu.Unlock()
			}
		}(link)
	}
	wg.Wait()

	return results, nil
}

func scrape1337xDetails(url string) (*domain.SearchResult, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	data := &domain.SearchResult{
		URL:  url,
		Name: strings.TrimSpace(doc.Find(".box-info-heading h1").Text()),
	}

	data.Magnet, _ = doc.Find(".clearfix ul li a").Attr("href")

	poster, _ := doc.Find("div.torrent-image img").Attr("src")
	if poster != "" {
		if strings.HasPrefix(poster, "http") {
			data.Poster = poster
		} else {
			data.Poster = "https:" + poster
		}
	}

	labels := []string{"Category", "Type", "Language", "Size", "UploadedBy", "Downloads", "LastChecked", "DateUploaded", "Seeders", "Leechers"}
	doc.Find("div .clearfix ul li > span").Each(func(i int, s *goquery.Selection) {
		if i < len(labels) {
			text := s.Text()
			switch labels[i] {
			case "Category":
				data.Category = text
			case "Type":
				data.Type = text
			case "Language":
				data.Language = text
			case "Size":
				data.Size = text
			case "UploadedBy":
				data.UploadedBy = text
			case "Downloads":
				data.Downloads = text
			case "LastChecked":
				data.LastChecked = text
			case "DateUploaded":
				data.DateUploaded = text
			case "Seeders":
				data.Seeders = text
			case "Leechers":
				data.Leechers = text
			}
		}
	})

	return data, nil
}
