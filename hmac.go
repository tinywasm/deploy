package deploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

type HMACValidator struct {
	secret []byte
}

func NewHMACValidator(secret string) *HMACValidator {
	return &HMACValidator{secret: []byte(secret)}
}

func (v *HMACValidator) ValidateRequest(payload []byte, signature string) error {
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("invalid signature format")
	}
	providedSig := strings.TrimPrefix(signature, "sha256=")
	providedBytes, err := hex.DecodeString(providedSig)
	if err != nil {
		return fmt.Errorf("invalid hex signature: %w", err)
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)

	if subtle.ConstantTimeCompare(providedBytes, expectedMAC) != 1 {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
