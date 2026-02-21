package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// healthResponse is the expected JSON body from the health endpoint.
type healthResponse struct {
	Status     string `json:"status"`
	CanRestart bool   `json:"can_restart"`
}

// HealthChecker checks if an application is healthy and ready for restart.
type HealthChecker interface {
	// Check returns nil if the app is alive and ready to restart.
	// It retries up to maxRetries times with interval between attempts.
	Check(url string, maxRetries int, interval time.Duration) error
}

// HTTPChecker is the production HealthChecker using net/http.
type HTTPChecker struct {
	client *http.Client
}

// NewChecker creates an HTTPChecker with a short per-request timeout.
func NewChecker() *HTTPChecker {
	return &HTTPChecker{client: &http.Client{Timeout: 5 * time.Second}}
}

// Check performs GET <url>/health up to maxRetries times.
// A 200 response with can_restart=true (or missing) is considered ready.
func (c *HTTPChecker) Check(url string, maxRetries int, interval time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		ready, err := c.checkOnce(url)
		if err != nil {
			lastErr = err
			continue
		}
		if !ready {
			lastErr = errors.New("app busy: can_restart=false")
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("health check failed after %d retries", maxRetries)
}

func (c *HTTPChecker) checkOnce(url string) (ready bool, err error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("health returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// If body is empty or not JSON, treat as healthy (simple /health endpoints)
	if len(body) == 0 {
		return true, nil
	}

	var hr healthResponse
	if err := json.Unmarshal(body, &hr); err != nil {
		// Non-JSON 200 → healthy
		return true, nil
	}

	// If can_restart field is absent (zero value = false default) but status is ok,
	// assume the app doesn't support the can_restart protocol → ready.
	if hr.Status == "ok" || hr.Status == "healthy" {
		return !hasCanRestartField(body) || hr.CanRestart, nil
	}
	return hr.CanRestart, nil
}

// hasCanRestartField detects if the JSON explicitly contains "can_restart".
func hasCanRestartField(data []byte) bool {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw["can_restart"]
	return ok
}
