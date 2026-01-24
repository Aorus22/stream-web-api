package torrent

import (
	"encoding/json"
	"fmt"
	"net/url"

	"torrent-stream/internal/domain"
)

// SearchNyaaSI searches NyaaSI via nyaaapi.onrender.com
func SearchNyaaSI(query string, page int) ([]*domain.SearchResult, error) {
	baseURL := "https://nyaaapi.onrender.com/nyaa"

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := PrepareRequest(u.String())
	if err != nil {
		return nil, err
	}

	client := GetHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status: %d", res.StatusCode)
	}

	// Decode as generic interface
	var raw interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var list []interface{}

	// Check if array
	if l, ok := raw.([]interface{}); ok {
		list = l
	} else if obj, ok := raw.(map[string]interface{}); ok {
		// Check common keys
		if d, ok := obj["data"].([]interface{}); ok {
			list = d
		} else if r, ok := obj["results"].([]interface{}); ok {
			list = r
		} else {
			// Fail-safe: try to return first array found in values
			found := false
			for _, v := range obj {
				if arr, ok := v.([]interface{}); ok {
					list = arr
					found = true
					break
				}
			}
			if !found {
				keys := make([]string, 0, len(obj))
				for k := range obj {
					keys = append(keys, k)
				}
				return nil, fmt.Errorf("unknown JSON object structure, keys: %v", keys)
			}
		}
	} else {
		return nil, fmt.Errorf("unknown JSON structure type")
	}

	var results []*domain.SearchResult

	for _, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Helper to try multiple keys
		getString := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := obj[k]; ok && v != nil {
					s := fmt.Sprintf("%v", v)
					if s != "" {
						return s
					}
				}
			}
			return ""
		}

		// Try to find Name
		name := getString("name", "title", "subject")

		// Try to find ID
		idVal := getString("id", "torrent_id", "torrentId")

		// Try to find Date
		date := getString("date", "created_at", "timestamp", "datetime")

		// Create result
		torrent := &domain.SearchResult{
			Name:         name,
			Category:     getString("category", "category_name"),
			URL:          fmt.Sprintf("https://nyaa.si/view/%s", idVal),
			Size:         getString("size", "filesize"),
			DateUploaded: date,
			Seeders:      getString("seeders", "seeders_count"),
			Leechers:     getString("leechers", "leechers_count"),
			Downloads:    getString("downloads", "completed_count"),
			Magnet:       getString("magnet", "magnet_link", "magnet_uri"),
		}

		if name == "" {
			keys := ""
			for k := range obj {
				keys += k + ","
			}
			torrent.Name = "DEBUG KEYS: " + keys
		}

		results = append(results, torrent)
	}

	return results, nil
}
