package repository

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/kolo/xmlrpc"

	"stream-web-api/internal/config"
	"stream-web-api/internal/domain/model"
)

const (
	OpenSubtitlesXMLRPCURL = "https://api.opensubtitles.org/xml-rpc"
	OpenSubtitlesUserAgent = "VLSub"
)

type OpenSubtitlesClient struct {
	endpoint  string
	userAgent string
	isProxy   bool
	proxyURL  string
	httpClient *http.Client
}

func NewOpenSubtitlesClient(cfg *config.OpenSubtitleConfig) *OpenSubtitlesClient {
	return &OpenSubtitlesClient{
		endpoint:  OpenSubtitlesXMLRPCURL,
		userAgent: OpenSubtitlesUserAgent,
		isProxy:   cfg.IsProxy,
		proxyURL:  cfg.ProxyURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *OpenSubtitlesClient) Search(query, lang string) ([]model.Subtitle, error) {
	if c.isProxy {
		return c.searchViaProxy(query, lang)
	}
	return c.searchViaXMLRPC(query, lang)
}

func (c *OpenSubtitlesClient) searchViaProxy(query, lang string) ([]model.Subtitle, error) {
	if c.proxyURL == "" {
		return nil, fmt.Errorf("proxy mode enabled but PROXY_OPENSUBTITLE_URL is not set")
	}

	u, err := url.Parse(c.proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	q := u.Query()
	q.Set("query", query)
	if lang == "" {
		lang = "eng,ind"
	}
	q.Set("lang", lang)
	u.RawQuery = q.Encode()

	log.Printf("DEBUG OS: Searching via proxy - URL: %s", u.String())

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(body))
	}

	var results []model.Subtitle
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse proxy response: %w", err)
	}

	log.Printf("DEBUG OS: Proxy returned %d results", len(results))
	return results, nil
}

func (c *OpenSubtitlesClient) searchViaXMLRPC(query, lang string) ([]model.Subtitle, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client, err := xmlrpc.NewClient(c.endpoint, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create XMLRPC client: %w", err)
	}
	_ = &http.Client{Transport: transport}

	var loginResp map[string]interface{}
	err = client.Call("LogIn", []interface{}{"", "", "en", c.userAgent}, &loginResp)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	log.Printf("DEBUG OS: loginResp = %+v", loginResp)

	tokenInterface, ok := loginResp["token"]
	if !ok || tokenInterface == nil {
		return nil, fmt.Errorf("no token in login response")
	}

	token, ok := tokenInterface.(string)
	if !ok {
		return nil, fmt.Errorf("invalid token type")
	}

	var num string
	defer client.Call("LogOut", []interface{}{token}, &num)

	if lang == "" {
		lang = "eng,ind"
	}

	searchQuery := map[string]string{
		"sublanguageid": lang,
		"query":         query,
	}
	log.Printf("DEBUG OS: Searching Subs - Query: '%s' Lang: '%s'", query, lang)

	var searchResp map[string]interface{}
	err = client.Call("SearchSubtitles", []interface{}{token, []interface{}{searchQuery}}, &searchResp)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	log.Printf("DEBUG OS: searchResp = %+v", searchResp)

	data := searchResp["data"]
	if data == nil {
		log.Printf("DEBUG OS: data is nil")
		return []model.Subtitle{}, nil
	}
	if b, active := data.(bool); active && !b {
		log.Printf("DEBUG OS: data is false boolean")
		return []model.Subtitle{}, nil
	}

	items, ok := data.([]interface{})
	if !ok {
		log.Printf("DEBUG OS: data is not array, type = %T", data)
		return []model.Subtitle{}, nil
	}

	log.Printf("DEBUG OS: found %d items", len(items))

	var results []model.Subtitle
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		res := model.Subtitle{
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
