package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      uint32 = 19 * 1024
	argon2Iterations  uint32 = 2
	argon2Parallelism uint8  = 1
	argon2SaltLength         = 16
	argon2KeyLength   uint32 = 32
)

// HashPassword hashes a password with Argon2id and returns a PHC-encoded
// string. The caller must not trim or otherwise normalize the password.
func HashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword reports whether password matches the PHC-encoded hash. A
// malformed hash or a password outside the accepted length returns false
// rather than an error so callers cannot tell the difference between a bad
// password and a damaged record.
func VerifyPassword(encoded, password string) (bool, error) {
	if err := validatePassword(password); err != nil {
		return false, nil
	}

	salt, expectedHash, params, err := parsePasswordHash(encoded)
	if err != nil {
		return false, nil
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(expectedHash)),
	)
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encoded string) ([]byte, []byte, argon2Params, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, argon2Params{}, errors.New("unsupported password hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, argon2Params{}, errors.New("unsupported password hash version")
	}

	var params argon2Params
	if _, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&params.memory,
		&params.iterations,
		&params.parallelism,
	); err != nil {
		return nil, nil, argon2Params{}, errors.New("invalid password hash parameters")
	}
	if params.memory == 0 || params.iterations == 0 || params.parallelism == 0 {
		return nil, nil, argon2Params{}, errors.New("invalid password hash parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return nil, nil, argon2Params{}, errors.New("invalid password hash salt")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) == 0 {
		return nil, nil, argon2Params{}, errors.New("invalid password hash")
	}
	return salt, hash, params, nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("%w: password must not be blank", ErrInvalidPassword)
	}

	length := utf8.RuneCountInString(password)
	if length < minPasswordLength || length > maxPasswordLength {
		return fmt.Errorf(
			"%w: password must be between %d and %d characters",
			ErrInvalidPassword,
			minPasswordLength,
			maxPasswordLength,
		)
	}
	return nil
}
