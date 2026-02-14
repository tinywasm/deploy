package deploy

import "github.com/zalando/go-keyring"

// SystemKeyManager implements KeyManager using the system keyring.
type SystemKeyManager struct{}

// NewSystemKeyManager creates a new SystemKeyManager.
func NewSystemKeyManager() *SystemKeyManager {
	return &SystemKeyManager{}
}

func (m *SystemKeyManager) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (m *SystemKeyManager) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
