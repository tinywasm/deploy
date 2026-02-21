package deploy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HealthStatus struct {
	Status     string `json:"status"`
	CanRestart bool   `json:"can_restart"`
}

type HealthChecker interface {
	Check(url string) (*HealthStatus, error)
}

type Checker struct {
	client *http.Client
}

func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Check performs a health check on the given URL.
func (c *Checker) Check(url string) (*HealthStatus, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return ParseHealthResponse(resp.Body)
}

func ParseHealthResponse(r io.Reader) (*HealthStatus, error) {
	var status HealthStatus
	if err := json.NewDecoder(r).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}
	return &status, nil
}
