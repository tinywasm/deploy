package deploy

// Store is a flat key-value store for deploy configuration and secrets.
// kvdb.KVStore satisfies this interface directly — no adapter needed.
//
// Keys used by deploy:
//
//	DEPLOY_METHOD       → "cloudflare" | "webhook" | "ssh"
//	DEPLOY_GITHUB_PAT   → GitHub Personal Access Token
//	DEPLOY_HMAC_SECRET  → HMAC-SHA256 secret for webhook validation
//	DEPLOY_SERVER_HOST  → host:port for webhook or SSH host
//	DEPLOY_SSH_USER     → SSH username
//	DEPLOY_SSH_KEY      → SSH private key path
//	CF_ACCOUNT_ID       → Cloudflare account ID
//	CF_PAGES_TOKEN      → Cloudflare scoped Pages:Edit token (auto-created)
//	CF_PROJECT          → Cloudflare Pages project name
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}
