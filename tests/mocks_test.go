package deploy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tinywasm/deploy"
)

// MockProcessManager records calls to Start and Stop.
type MockProcessManager struct {
	mu      sync.Mutex
	Started []string
	Stopped []string
}

func NewMockProcessManager() *MockProcessManager {
	return &MockProcessManager{
		Started: make([]string, 0),
		Stopped: make([]string, 0),
	}
}

func (m *MockProcessManager) Start(exePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Started = append(m.Started, exePath)
	return nil
}

func (m *MockProcessManager) Stop(exeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Stopped = append(m.Stopped, exeName)
	return nil
}

// MockDownloader records calls to Download and simulates file creation.
type MockDownloader struct {
	mu           sync.Mutex
	Downloaded   []string // url -> dest
	ShouldFail   bool
	ShouldFailAs int // Status code
}

func NewMockDownloader() *MockDownloader {
	return &MockDownloader{
		Downloaded: make([]string, 0),
	}
}

func (m *MockDownloader) Download(url, dest, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ShouldFail {
		return fmt.Errorf("mock download failed")
	}
	m.Downloaded = append(m.Downloaded, fmt.Sprintf("%s -> %s", url, dest))

	// Create dummy file at dest
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, []byte("mock downloaded content"), 0644)
}

// MockKeyManager records calls to Get and Set.
type MockKeyManager struct {
	mu      sync.Mutex
	Secrets map[string]string
}

func NewMockKeyManager() *MockKeyManager {
	return &MockKeyManager{
		Secrets: make(map[string]string),
	}
}

func (m *MockKeyManager) Get(service, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", service, user)
	val, ok := m.Secrets[key]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return val, nil
}

func (m *MockKeyManager) Set(service, user, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", service, user)
	m.Secrets[key] = password
	return nil
}

// MockHealthChecker records calls to Check.
type MockHealthChecker struct {
	mu         sync.Mutex
	Responses  map[string]*deploy.HealthStatus
	ShouldFail bool
}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		Responses: make(map[string]*deploy.HealthStatus),
	}
}

func (m *MockHealthChecker) Check(url string) (*deploy.HealthStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ShouldFail {
		return nil, fmt.Errorf("mock health check failed")
	}
	status, ok := m.Responses[url]
	if !ok {
		return &deploy.HealthStatus{Status: "ok", CanRestart: true}, nil
	}
	return status, nil
}
