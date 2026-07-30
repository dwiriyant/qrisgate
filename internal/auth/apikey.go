package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const apiKeyPrefix = "qg_"

// GenerateAPIKey returns a new raw API key with prefix qg_.
func GenerateAPIKey() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return apiKeyPrefix + hex.EncodeToString(b[:]), nil
}

// HashAPIKey returns a hex-encoded SHA-256 of the raw key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// VerifyAPIKey compares a raw key to a stored hash.
func VerifyAPIKey(raw, hash string) bool {
	return hmac.Equal([]byte(HashAPIKey(raw)), []byte(hash))
}

// SignWebhook computes HMAC-SHA256 hex for webhook body.
func SignWebhook(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// FormatStoredKey documents storage format (hash only).
func FormatStoredKey(hash string) string {
	return fmt.Sprintf("sha256:%s", hash)
}
