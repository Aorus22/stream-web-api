package torrent

import (
	"errors"
	"sort"
	"sync"
	"torrent-stream/internal/domain"
)

// SearchProvider is the interface for torrent search providers
type SearchProvider func(query string, page int) ([]*domain.SearchResult, error)

var providers = map[string]SearchProvider{
	"1337x":          Search1337x,
	"bitsearch":      SearchBitSearch,
	"ettv":           SearchEttvCentral,
	"eztv":           SearchEzTV,
	"glotorrents":    SearchGloTorrents,
	"kickass":        SearchKickAss,
	"limetorrent":    SearchLimeTorrent,
	"magnetdl":       SearchMagnetDL,
	"nyaasi":         SearchNyaaSI,
	"piratebay":      SearchPirateBay,
	"rarbg":          SearchRarbg,
	"torlock":        SearchTorLock,
	"torrentfunk":    SearchTorrentFunk,
	"torrentgalaxy":  SearchTorrentGalaxy,
	"torrentproject": SearchTorrentProject,
	"yts":            SearchYTS,
	"zooqle":         SearchZooqle,
}

// Search executes a search using the specified provider
func Search(provider, query string, page int) ([]*domain.SearchResult, error) {
	if provider == "all" {
		return SearchAll(query, page)
	}

	if p, ok := providers[provider]; ok {
		return p(query, page)
	}
	return nil, errors.New("provider not found")
}

// SearchAll searches all providers concurrently
func SearchAll(query string, page int) ([]*domain.SearchResult, error) {
	var results []*domain.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(searchFunc SearchProvider) {
			defer wg.Done()
			// Ignore errors from individual providers to keep partial results
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

// GetProviders returns a list of available providers
func GetProviders() []string {
	var list []string
	for k := range providers {
		list = append(list, k)
	}
	sort.Strings(list)
	list = append([]string{"all"}, list...) // Add "all" option at the beginning
	return list
}
