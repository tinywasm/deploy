package deploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

// HMACValidator validates GitHub-style HMAC-SHA256 webhook signatures.
type HMACValidator struct {
	secret []byte
}

// NewHMACValidator creates a validator with the given secret.
func NewHMACValidator(secret string) *HMACValidator {
	return &HMACValidator{secret: []byte(secret)}
}

// Validate checks that signature matches HMAC-SHA256(payload, secret).
// Signature must have the format "sha256=<hex>".
func (v *HMACValidator) Validate(payload []byte, signature string) error {
	if !strings.HasPrefix(signature, "sha256=") {
		return errors.New("invalid signature format")
	}
	sigHex := strings.TrimPrefix(signature, "sha256=")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return errors.New("invalid hex in signature")
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	if subtle.ConstantTimeCompare(sigBytes, expected) != 1 {
		return errors.New("signature mismatch")
	}
	return nil
}

// SignPayload generates a GitHub-style HMAC-SHA256 signature for payload using secret.
// Useful for testing and for the GitHub Action side.
func SignPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
