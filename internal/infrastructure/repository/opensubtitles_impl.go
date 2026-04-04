package repository

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/kolo/xmlrpc"

	"stream-web-api/internal/domain/model"
)

const (
	OpenSubtitlesXMLRPCURL = "https://api.opensubtitles.org/xml-rpc"
	OpenSubtitlesUserAgent = "VLSub"
)

type OpenSubtitlesClient struct {
	endpoint  string
	userAgent string
}

func NewOpenSubtitlesClient() *OpenSubtitlesClient {
	return &OpenSubtitlesClient{
		endpoint:  OpenSubtitlesXMLRPCURL,
		userAgent: OpenSubtitlesUserAgent,
	}
}

func (c *OpenSubtitlesClient) Search(query, lang string) ([]model.Subtitle, error) {
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
