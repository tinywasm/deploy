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
	// Input: 1 (Auto Setup) -> HMAC (min 32) -> PAT (ghp_)
	input := "1\nmyhmac_secret_must_be_at_least_32_chars_long\nghp_mygithubpat\n"
	stdin := bytes.NewBufferString(input)
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
	expectedHMAC := "myhmac_secret_must_be_at_least_32_chars_long"
	if hmac != expectedHMAC {
		t.Errorf("expected hmac '%s', got '%s'", expectedHMAC, hmac)
	}

	pat, err := keys.Get("github", "pat")
	if err != nil {
		t.Errorf("github pat not found")
	}
	expectedPAT := "ghp_mygithubpat"
	if pat != expectedPAT {
		t.Errorf("expected pat '%s', got '%s'", expectedPAT, pat)
	}
}

func TestWizard_Run_Partial(t *testing.T) {
	keys := NewMockKeyManager()
	// If one key exists but other missing, Run() calls MainLoop which prompts for both.
	// We simulate this by only setting one key.
	keys.Set("github", "pat", "existing_pat")

	// Input: 1 (Auto Setup) -> HMAC -> PAT
	input := "1\nnewhmac_secret_must_be_at_least_32_chars_long\nghp_newpat\n"
	stdin := bytes.NewBufferString(input)
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
	expectedHMAC := "newhmac_secret_must_be_at_least_32_chars_long"
	if hmac != expectedHMAC {
		t.Errorf("expected hmac '%s', got '%s'", expectedHMAC, hmac)
	}

	pat, err := keys.Get("github", "pat")
	if err != nil {
		t.Errorf("github pat not found")
	}
	expectedPAT := "ghp_newpat"
	if pat != expectedPAT {
		t.Errorf("expected pat '%s', got '%s'", expectedPAT, pat)
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
