package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	Updater ConfigUpdater `yaml:"updater"`
	Apps    []AppConfig   `yaml:"apps"`
}

// ConfigUpdater holds updater-specific configuration.
type ConfigUpdater struct {
	Port     int           `yaml:"port"` // default: 8080
	LogLevel string        `yaml:"log_level"`
	LogFile  string        `yaml:"log_file"`
	TempDir  string        `yaml:"temp_dir"`
	Retry    RetryConfig   `yaml:"retry"`
}

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts"`
	Delay       time.Duration `yaml:"delay"`
}

// AppConfig represents a single application configuration.
type AppConfig struct {
	Name              string         `yaml:"name"`
	Version           string         `yaml:"version"`
	Executable        string         `yaml:"executable"`
	Path              string         `yaml:"path"`
	Port              int            `yaml:"port"`
	HealthEndpoint    string         `yaml:"health_endpoint"`
	HealthTimeout     time.Duration  `yaml:"health_timeout"`
	StartupDelay      time.Duration  `yaml:"startup_delay"`
	BusyRetryInterval time.Duration  `yaml:"busy_retry_interval"` // default: 10s
	BusyTimeout       time.Duration  `yaml:"busy_timeout"`        // default: 5m
	Rollback          RollbackConfig `yaml:"rollback"`
}

// RollbackConfig holds rollback configuration.
type RollbackConfig struct {
	Enabled               bool `yaml:"enabled"`
	KeepVersions          int  `yaml:"keep_versions"` // Only -older
	AutoRollbackOnFailure bool `yaml:"auto_rollback_on_failure"`
}

// Load loads the configuration from the specified path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist?
			// Guide says: "Attempt to load config.yaml (create default if missing)."
			// But creating default is part of `Deploy.Run` logic usually.
			// Here we should probably return error or default config if not found?
			// Let's return error and handle it in `Deploy.Run`.
			return nil, err
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if config.Updater.Port == 0 {
		config.Updater.Port = 8080
	}
	if config.Updater.TempDir == "" {
		config.Updater.TempDir = filepath.Join(os.TempDir(), "deploy")
	}

	for i := range config.Apps {
		if config.Apps[i].BusyRetryInterval == 0 {
			config.Apps[i].BusyRetryInterval = 10 * time.Second
		}
		if config.Apps[i].BusyTimeout == 0 {
			config.Apps[i].BusyTimeout = 5 * time.Minute
		}
	}

	return &config, nil
}
