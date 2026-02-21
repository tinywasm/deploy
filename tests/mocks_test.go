// Package tests contains unit and integration tests for tinywasm/deploy.
// All external dependencies are mocked — no real processes, network calls, or keyring access.
package tests

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tinywasm/deploy"
)

// mockKeyManager simulates tinywasm/keyring without touching the OS keyring.
type mockKeyManager struct {
	hmacSecret string
	pat        string
	configured bool
}

func (m *mockKeyManager) GetHMACSecret() (string, error) {
	if m.hmacSecret == "" {
		return "", errors.New("hmac secret not set")
	}
	return m.hmacSecret, nil
}

func (m *mockKeyManager) GetGitHubPAT() (string, error) {
	if m.pat == "" {
		return "", errors.New("PAT not set")
	}
	return m.pat, nil
}

func (m *mockKeyManager) IsConfigured() bool { return m.configured }

// mockProcessManager records Start/Stop calls without executing OS commands.
type mockProcessManager struct {
	started []string
	stopped []string
	startErr error
	stopErr  error
}

func (m *mockProcessManager) Stop(service string) error {
	m.stopped = append(m.stopped, service)
	return m.stopErr
}

func (m *mockProcessManager) Start(exePath string) error {
	m.started = append(m.started, exePath)
	return m.startErr
}

// mockDownloader simulates binary downloads without network.
type mockDownloader struct {
	err error
}

func (m *mockDownloader) Download(url, dest, token string) error {
	return m.err
}

// mockChecker simulates health checks without network.
type mockChecker struct {
	responses []error // popped in order; last is repeated if exhausted
}

func (m *mockChecker) Check(url string, maxRetries int, interval time.Duration) error {
	if len(m.responses) == 0 {
		return nil
	}
	r := m.responses[0]
	if len(m.responses) > 1 {
		m.responses = m.responses[1:]
	}
	return r
}

// mockFileOps records rename/remove calls without touching the filesystem.
type mockFileOps struct {
	renames []string // "old->new"
	removes []string
	renameErr error
}

func (m *mockFileOps) Rename(oldPath, newPath string) error {
	m.renames = append(m.renames, oldPath+"->"+newPath)
	return m.renameErr
}

func (m *mockFileOps) Remove(path string) error {
	m.removes = append(m.removes, path)
	return nil
}

// testRoundTripper allows httptest-like responses for mockChecker used in handler tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newOKResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}
}

func newJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io_nopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// io_nopCloser wraps a strings.Reader with a no-op Close.
type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

func io_nopCloser(r *strings.Reader) *nopCloser { return &nopCloser{r} }

// testConfig returns a minimal Config for tests.
func testConfig() *deploy.Config {
	return &deploy.Config{
		Mode:    "webhook",
		Port:    9000,
		TempDir: "/tmp",
		Apps: []deploy.AppConfig{
			{
				Name:        "myapp",
				Service:     "myapp.service",
				Executable:  "myapp",
				Path:        "/srv/myapp",
				HealthURL:   "http://localhost:8080/health",
				Rollback:    true,
				StartupWait: 0,
				HealthRetry: 2,
				Version:     "v1.0.0",
			},
		},
	}
}
