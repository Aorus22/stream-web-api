package repository

import (
	"encoding/json"
	"fmt"
	"net/url"

	"stream-web-api/internal/domain/model"
)

func SearchNyaaSI(query string, page int) ([]*model.SearchResult, error) {
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

	var raw interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var list []interface{}

	if l, ok := raw.([]interface{}); ok {
		list = l
	} else if obj, ok := raw.(map[string]interface{}); ok {
		if d, ok := obj["data"].([]interface{}); ok {
			list = d
		} else if r, ok := obj["results"].([]interface{}); ok {
			list = r
		} else {
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

	var results []*model.SearchResult

	for _, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

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

		name := getString("name", "title", "subject")

		idVal := getString("id", "torrent_id", "torrentId")

		date := getString("date", "created_at", "timestamp", "datetime")

		torrent := &model.SearchResult{
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
