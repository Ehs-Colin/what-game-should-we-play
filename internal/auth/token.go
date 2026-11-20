package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenByteLength = 32

// NewSessionToken returns a new opaque session token and its SHA-256 hash. The
// raw token is sent to the client in a cookie; only the hash is stored.
func NewSessionToken() (string, []byte, error) {
	tokenBytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	return token, HashSessionToken(token), nil
}

// HashSessionToken returns the SHA-256 hash stored in app_sessions.
func HashSessionToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

// NewCSRFToken returns a new opaque CSRF token.
func NewCSRFToken() (string, error) {
	tokenBytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}
