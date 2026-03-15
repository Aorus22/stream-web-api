package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"time"
)

type Client struct {
	clientID     string
	clientSecret string
	refreshToken string
}

func NewClient(clientID, clientSecret, refreshToken string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (c *Client) getAccessToken() (string, error) {
	data := map[string]string{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"refresh_token": c.refreshToken,
		"grant_type":    "refresh_token",
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to refresh token: %s", string(body))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}

	return tr.AccessToken, nil
}

type ProgressReader struct {
	io.Reader
	Total      int64
	ReadBytes  int64
	OnProgress func(float64)
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.ReadBytes += int64(n)
	if pr.Total > 0 && pr.OnProgress != nil {
		pr.OnProgress(float64(pr.ReadBytes) / float64(pr.Total) * 100.0)
	}
	return
}

func (c *Client) Upload(ctx context.Context, filePath string, filename string, onProgress func(float64)) (string, string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return "", "", fmt.Errorf("auth error: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	stat, _ := file.Stat()
	fileSize := stat.Size()

	// Gunakan Pipe agar data mengalir langsung dari disk ke koneksi internet tanpa mampir ke RAM
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		// Metadata part
		metadata := map[string]interface{}{
			"name": filename,
		}
		metadataJSON, _ := json.Marshal(metadata)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", "application/json; charset=UTF-8")
		part, _ := writer.CreatePart(h)
		part.Write(metadataJSON)

		// Media part
		h = make(textproto.MIMEHeader)
		h.Set("Content-Type", "application/octet-stream")
		mediaPart, _ := writer.CreatePart(h)

		// Track progress saat copy
		progReader := &ProgressReader{
			Reader:     file,
			Total:      fileSize,
			OnProgress: onProgress,
		}
		io.Copy(mediaPart, progReader)
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&fields=id,webViewLink", pr)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	// Timeout lebih panjang untuk file besar
	client := &http.Client{Timeout: 120 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID          string `json:"id"`
		WebViewLink string `json:"webViewLink"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.ID, result.WebViewLink, nil
}
