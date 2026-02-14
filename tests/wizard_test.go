package deploy_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/deploy"
)

func TestWizard_Run(t *testing.T) {
	keys := NewMockKeyManager()
	stdin := bytes.NewBufferString("myhmac\nmygithubpat\n")
	stdout := new(bytes.Buffer)

	wizard := &deploy.Wizard{
		Keys:   keys,
		Stdin:  stdin,
		Stdout: stdout,
	}

	if err := wizard.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify secrets
	hmac, err := keys.Get("deploy", "hmac_secret")
	if err != nil {
		t.Errorf("hmac secret not found")
	}
	if hmac != "myhmac" {
		t.Errorf("expected hmac 'myhmac', got '%s'", hmac)
	}

	pat, err := keys.Get("github", "pat")
	if err != nil {
		t.Errorf("github pat not found")
	}
	if pat != "mygithubpat" {
		t.Errorf("expected pat 'mygithubpat', got '%s'", pat)
	}
}

func TestWizard_Run_Partial(t *testing.T) {
	keys := NewMockKeyManager()
	keys.Set("github", "pat", "existing_pat")

	// Missing hmac. Should prompt for both.
	stdin := bytes.NewBufferString("newhmac\nnewpat\n")
	stdout := new(bytes.Buffer)

	wizard := &deploy.Wizard{
		Keys:   keys,
		Stdin:  stdin,
		Stdout: stdout,
	}

	if err := wizard.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	hmac, err := keys.Get("deploy", "hmac_secret")
	if err != nil {
		t.Errorf("hmac secret not found")
	}
	if hmac != "newhmac" {
		t.Errorf("expected hmac 'newhmac', got '%s'", hmac)
	}

	pat, err := keys.Get("github", "pat")
	if err != nil {
		t.Errorf("github pat not found")
	}
	if pat != "newpat" {
		t.Errorf("expected pat 'newpat', got '%s'", pat)
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := deploy.CreateDefaultConfig(configPath); err != nil {
		t.Fatalf("CreateDefaultConfig() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("config file is empty")
	}
}
