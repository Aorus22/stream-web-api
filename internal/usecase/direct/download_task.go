package direct

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"
)

type downloadTask struct {
	id       int
	url      string
	filePath string

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

type progressCallback func(downloadedBytes int64, totalBytes int64, status string)

func (t *downloadTask) run(cb progressCallback) error {
	defer close(t.done)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 0, // allow large downloads; cancellation is via context
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		cb(0, 0, "failed")
		return &httpError{statusCode: resp.StatusCode}
	}

	totalBytes := resp.ContentLength
	if totalBytes < 0 {
		totalBytes = 0
	}

	out, err := os.Create(t.filePath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	buf := make([]byte, 1024*256)
	var downloaded int64
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
		}

		if time.Since(lastReport) >= 500*time.Millisecond {
			cb(downloaded, totalBytes, "downloading")
			lastReport = time.Now()
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	cb(downloaded, totalBytes, "completed")
	return nil
}

type httpError struct {
	statusCode int
}

func (e *httpError) Error() string {
	return http.StatusText(e.statusCode)
}

