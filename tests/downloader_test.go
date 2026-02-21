package deploy_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/deploy"
)

func TestDownload_Success(t *testing.T) {
	content := "test content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/octet-stream" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		io.WriteString(w, content)
	}))
	defer ts.Close()

	destDir := t.TempDir()
	destFile := filepath.Join(destDir, "downloaded.txt")

	d := deploy.NewDownloader()
	if err := d.Download(ts.URL, destFile, "valid-token"); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected content '%s', got '%s'", content, string(data))
	}
}

func TestDownload_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	destDir := t.TempDir()
	destFile := filepath.Join(destDir, "failed.txt")

	d := deploy.NewDownloader()
	if err := d.Download(ts.URL, destFile, "token"); err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestMock_Download(t *testing.T) {
	mock := NewMockDownloader()
	url := "http://example.com/file"
	dest := "/tmp/file"
	token := "token"

	if err := mock.Download(url, dest, token); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if len(mock.Downloaded) != 1 || mock.Downloaded[0] != fmt.Sprintf("%s -> %s", url, dest) {
		t.Errorf("expected Downloaded to contain %s -> %s, got %v", url, dest, mock.Downloaded)
	}

	mock.ShouldFail = true
	if err := mock.Download(url, dest, token); err == nil {
		t.Fatal("expected error, got nil")
	}
}
