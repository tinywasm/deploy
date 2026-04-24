package deploy

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// Store is a flat key-value store for deploy configuration and secrets.
// kvdb.KVStore satisfies this interface directly — no adapter needed.
//
// Keys used by deploy:
//
//	DEPLOY_METHOD       → "cloudflarePages" | "cloudflareWorker" | "webhook" | "ssh"
//	DEPLOY_GITHUB_PAT   → GitHub Personal Access Token
//	DEPLOY_HMAC_SECRET  → HMAC-SHA256 secret for webhook validation
//	DEPLOY_SERVER_HOST  → host:port for webhook or SSH host
//	DEPLOY_SSH_USER     → SSH username
//	DEPLOY_SSH_KEY      → SSH private key path/content
//	CF_ACCOUNT_ID       → Cloudflare account ID
//	CF_PAGES_TOKEN      → Cloudflare scoped Pages:Edit token (auto-created)
//	CF_PROJECT          → Cloudflare project name
//	CF_WORKER_TOKEN     → Cloudflare scoped Workers:Edit token
// KeyringServiceName is the service name used for storing secrets in the OS keyring.
const KeyringServiceName = "tinywasm-deploy"

var sensitiveKeys = map[string]bool{
	"DEPLOY_GITHUB_PAT":  true,
	"DEPLOY_HMAC_SECRET": true,
	"DEPLOY_SSH_KEY":     true,
	"CF_PAGES_TOKEN":     true,
	"CF_WORKER_TOKEN":    true,
}

// isSensitive reports whether the given key contains sensitive information
// that should be stored in the OS keyring.
func isSensitive(key string) bool {
	return sensitiveKeys[key] || strings.HasPrefix(key, "goflare/")
}

// SecureStore wraps a base Store and routes sensitive keys securely to the OS keyring.
// Minimal interface: no adapter needed for the base KVStore.
type SecureStore struct {
	base Store
}

// NewSecureStore initializes a new SecureStore wrapping the given base store.
func NewSecureStore(base Store) *SecureStore {
	return &SecureStore{base: base}
}

// Get retrieves a key. Sensitive keys are fetched only from the keyring.
func (s *SecureStore) Get(key string) (string, error) {
	if isSensitive(key) {
		val, err := keyring.Get(KeyringServiceName, key)
		if err != nil {
			return "", fmt.Errorf("secure store: key %q not found in keyring: %w", key, err)
		}
		return val, nil
	}
	return s.base.Get(key)
}

// Set stores a key. Sensitive keys are saved only to the keyring,
// and concurrently wiped from the base store to prevent plaintext leaks.
func (s *SecureStore) Set(key, value string) error {
	if isSensitive(key) {
		if value == "" {
			return keyring.Delete(KeyringServiceName, key)
		}
		if err := keyring.Set(KeyringServiceName, key, value); err != nil {
			return fmt.Errorf("secure store: failed to save %q to keyring: %w", key, err)
		}
		// Wipe from base store if it existed previously
		_ = s.base.Set(key, "")
		return nil
	}
	return s.base.Set(key, value)
}
