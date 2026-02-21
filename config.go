package deploy

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level deploy configuration (deploy.yaml).
type Config struct {
	Mode    string      // "webhook" or "ssh"
	Port    int         // webhook listener port (default: 9000)
	LogFile string      `yaml:"log_file"`
	TempDir string      `yaml:"temp_dir"`
	Apps    []AppConfig `yaml:"apps"`

	// SSH mode fields
	SSHHost string `yaml:"ssh_host"`
	SSHUser string `yaml:"ssh_user"`
	SSHKey  string `yaml:"ssh_key"` // path to private key file
}

// AppConfig holds per-application deployment settings.
type AppConfig struct {
	Name        string        // unique name (must match payload)
	Service     string        // systemd service or Windows process name
	Executable  string        // binary filename (e.g. myapp or myapp.exe)
	Path        string        // absolute path to deploy directory
	GithubRepo  string        `yaml:"github_repo"`  // "owner/repo"
	HealthURL   string        `yaml:"health_url"`   // e.g. http://localhost:8080/health
	Port        int           // app port (optional, used for health if HealthURL is empty)
	Rollback    bool          // enable automatic rollback (default: true)
	StartupWait time.Duration `yaml:"startup_wait"` // wait after start before health check (default: 3s)
	HealthRetry int           `yaml:"health_retry"` // max retries on health check (default: 5)
	Version     string        // current deployed version (updated after successful deploy)
}

const defaultPort = 9000
const defaultStartupWait = 3 * time.Second
const defaultHealthRetry = 5

// Load reads a config.yaml file and returns a validated Config.
// Missing optional fields receive sensible defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	setDefaults(&c)
	return &c, nil
}

// Save writes c to path as YAML.
func Save(path string, c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func setDefaults(c *Config) {
	if c.Mode == "" {
		c.Mode = "webhook"
	}
	if c.Port == 0 {
		c.Port = defaultPort
	}
	if c.TempDir == "" {
		c.TempDir = os.TempDir()
	}
	for i := range c.Apps {
		if c.Apps[i].StartupWait == 0 {
			c.Apps[i].StartupWait = defaultStartupWait
		}
		if c.Apps[i].HealthRetry == 0 {
			c.Apps[i].HealthRetry = defaultHealthRetry
		}
	}
}
