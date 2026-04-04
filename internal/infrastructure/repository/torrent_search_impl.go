package repository

import (
	"errors"
	"sort"
	"sync"
	"stream-web-api/internal/domain/model"
)

type SearchProvider func(query string, page int) ([]*model.SearchResult, error)

var providers = map[string]SearchProvider{
	"nyaasi":      SearchNyaaSI,
	"piratebay":   SearchPirateBay,
	"torrentfunk": SearchTorrentFunk,
}

func Search(provider, query string, page int) ([]*model.SearchResult, error) {
	if provider == "all" {
		return SearchAll(query, page)
	}

	if p, ok := providers[provider]; ok {
		return p(query, page)
	}
	return nil, errors.New("provider not found")
}

func SearchAll(query string, page int) ([]*model.SearchResult, error) {
	var results []*model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(searchFunc SearchProvider) {
			defer wg.Done()
			if res, err := searchFunc(query, page); err == nil {
				mu.Lock()
				results = append(results, res...)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	return results, nil
}

func GetProviders() []string {
	var list []string
	for k := range providers {
		list = append(list, k)
	}
	sort.Strings(list)
	list = append([]string{"all"}, list...)
	return list
}
