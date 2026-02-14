package deploy

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Wizard struct {
	Keys   KeyManager
	Stdin  io.Reader
	Stdout io.Writer
}

func NewWizard(keys KeyManager) *Wizard {
	return &Wizard{
		Keys:   keys,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
	}
}

func (w *Wizard) Run() error {
	// check if configured
	_, patErr := w.Keys.Get("github", "pat")
	_, hmacErr := w.Keys.Get("deploy", "hmac_secret")

	if patErr == nil && hmacErr == nil {
		// Already configured
		return nil
	}

	scanner := bufio.NewScanner(w.Stdin)

	fmt.Fprintln(w.Stdout, "Deploy Setup Wizard")
	fmt.Fprintln(w.Stdout, "-------------------")

	// Prompt for missing keys or all keys?
	// Simpler to prompt for all to ensure consistency.
	// Or check individually.

	// Enter HMAC Secret
	fmt.Fprint(w.Stdout, "Enter HMAC Secret: ")
	if !scanner.Scan() {
		return scanner.Err()
	}
	hmacSecret := strings.TrimSpace(scanner.Text())
	if hmacSecret == "" {
		// If existing secret is valid, maybe allow empty input to keep it?
		// But for now strict.
		return fmt.Errorf("HMAC secret cannot be empty")
	}

	// Enter GitHub PAT
	fmt.Fprint(w.Stdout, "Enter GitHub PAT: ")
	if !scanner.Scan() {
		return scanner.Err()
	}
	githubPat := strings.TrimSpace(scanner.Text())
	if githubPat == "" {
		return fmt.Errorf("GitHub PAT cannot be empty")
	}

	// Save secrets
	if err := w.Keys.Set("deploy", "hmac_secret", hmacSecret); err != nil {
		return fmt.Errorf("failed to save HMAC secret: %w", err)
	}

	if err := w.Keys.Set("github", "pat", githubPat); err != nil {
		return fmt.Errorf("failed to save GitHub PAT: %w", err)
	}

	return nil
}

// CreateDefaultConfig creates a default config file at the given path.
func CreateDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	defaultConfig := `updater:
  port: 8080
  log_level: info
  temp_dir: ./temp

apps: []
`
	return os.WriteFile(path, []byte(defaultConfig), 0644)
}
