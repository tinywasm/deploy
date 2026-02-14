package deploy_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/tinywasm/deploy"
)

func TestValidate_ValidSignature(t *testing.T) {
	secret := "mysecret"
	validator := deploy.NewHMACValidator(secret)
	payload := []byte(`{"foo":"bar"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if err := validator.ValidateRequest(payload, signature); err != nil {
		t.Fatalf("ValidateRequest() error = %v", err)
	}
}

func TestValidate_InvalidSignature(t *testing.T) {
	secret := "mysecret"
	validator := deploy.NewHMACValidator(secret)
	payload := []byte(`{"foo":"bar"}`)
	signature := "sha256=" + hex.EncodeToString([]byte("invalid_signature_bytes_should_be_hmac"))

	if err := validator.ValidateRequest(payload, signature); err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

func TestValidate_MalformedFormat(t *testing.T) {
	validator := deploy.NewHMACValidator("secret")
	if err := validator.ValidateRequest([]byte("data"), "invalid_format"); err == nil {
		t.Fatal("expected error for malformed signature, got nil")
	}
}

func TestValidate_InvalidHex(t *testing.T) {
	validator := deploy.NewHMACValidator("secret")
	if err := validator.ValidateRequest([]byte("data"), "sha256=zzzz"); err == nil {
		t.Fatal("expected error for invalid hex, got nil")
	}
}
