package deploy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader fetches a release binary from a URL.
type Downloader interface {
	// Download saves the content at url to destPath, authenticating with token.
	Download(url, destPath, token string) error
}

// HTTPDownloader is the production Downloader using net/http.
type HTTPDownloader struct {
	client *http.Client
}

// NewDownloader creates an HTTPDownloader with a generous timeout for large binaries.
func NewDownloader() *HTTPDownloader {
	return &HTTPDownloader{client: &http.Client{Timeout: 10 * time.Minute}}
}

// Download fetches url with Bearer <token> and writes to destPath.
func (d *HTTPDownloader) Download(url, destPath, token string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("download mkdir: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("download fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: server returned %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("download create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("download write: %w", err)
	}

	// Make executable on Unix
	return os.Chmod(destPath, 0755)
}
