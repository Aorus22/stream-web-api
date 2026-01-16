package opensubtitles

import (
	"fmt"
	"log"

	"github.com/kolo/xmlrpc"

	"torrent-stream/internal/domain"
)

const (
	XMLRPC_URL = "https://api.opensubtitles.org/xml-rpc"
	USER_AGENT = "VLSub"
)

// Client wraps OpenSubtitles API
type Client struct {
	endpoint  string
	userAgent string
}

// NewClient creates a new OpenSubtitles client
func NewClient() *Client {
	return &Client{
		endpoint:  XMLRPC_URL,
		userAgent: USER_AGENT,
	}
}

// Search searches for subtitles
func (c *Client) Search(query, lang string) ([]domain.Subtitle, error) {
	client, err := xmlrpc.NewClient(c.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create XMLRPC client: %w", err)
	}

	// Login
	var loginResp map[string]interface{}
	err = client.Call("LogIn", []interface{}{"", "", "en", c.userAgent}, &loginResp)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	tokenInterface, ok := loginResp["token"]
	if !ok || tokenInterface == nil {
		return nil, fmt.Errorf("no token in login response")
	}

	token, ok := tokenInterface.(string)
	if !ok {
		return nil, fmt.Errorf("invalid token type")
	}

	// Logout on return
	var num string
	defer client.Call("LogOut", []interface{}{token}, &num)

	// Search
	if lang == "" {
		lang = "eng,ind"
	}

	searchQuery := map[string]string{
		"sublanguageid": lang,
		"query":         query,
	}
	log.Printf("DEBUG: Searching Subs - Query: '%s' Lang: '%s'", query, lang)

	var searchResp map[string]interface{}
	err = client.Call("SearchSubtitles", []interface{}{token, []interface{}{searchQuery}}, &searchResp)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	data := searchResp["data"]
	if data == nil {
		return []domain.Subtitle{}, nil
	}
	if b, active := data.(bool); active && !b {
		return []domain.Subtitle{}, nil
	}

	items, ok := data.([]interface{})
	if !ok {
		return []domain.Subtitle{}, nil
	}

	var results []domain.Subtitle
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		res := domain.Subtitle{
			IDMovie:         fmt.Sprintf("%v", m["IDMovie"]),
			IDSubtitleFile:  fmt.Sprintf("%v", m["IDSubtitleFile"]),
			MovieName:       fmt.Sprintf("%v", m["MovieName"]),
			SubFileName:     fmt.Sprintf("%v", m["SubFileName"]),
			LanguageName:    fmt.Sprintf("%v", m["LanguageName"]),
			ZipDownloadLink: fmt.Sprintf("%v", m["ZipDownloadLink"]),
			SubDownloadLink: fmt.Sprintf("%v", m["SubDownloadLink"]),
		}
		results = append(results, res)
	}

	return results, nil
}
