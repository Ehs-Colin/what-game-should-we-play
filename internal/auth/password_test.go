package auth

import (
	"errors"
	"strings"
	"testing"
)

const passwordTestPassword = "correct horse battery staple"

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword(passwordTestPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Fatalf("encoded hash = %q, want an argon2id PHC string", encoded)
	}

	valid, err := VerifyPassword(encoded, passwordTestPassword)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !valid {
		t.Fatal("verify password = false, want true")
	}

	valid, err = VerifyPassword(encoded, passwordTestPassword+"x")
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if valid {
		t.Fatal("verify wrong password = true, want false")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	first, err := HashPassword(passwordTestPassword)
	if err != nil {
		t.Fatalf("hash first password: %v", err)
	}
	second, err := HashPassword(passwordTestPassword)
	if err != nil {
		t.Fatalf("hash second password: %v", err)
	}
	if first == second {
		t.Fatal("two hashes of the same password are identical")
	}

	for index, encoded := range []string{first, second} {
		valid, err := VerifyPassword(encoded, passwordTestPassword)
		if err != nil || !valid {
			t.Fatalf("hash %d did not verify: valid=%v err=%v", index, valid, err)
		}
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	validHash, err := HashPassword(passwordTestPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name   string
		encode string
	}{
		{"empty", ""},
		{"plain text", "not-a-password-hash"},
		{"wrong algorithm", strings.Replace(validHash, "argon2id", "argon2i", 1)},
		{"wrong version", strings.Replace(validHash, "v=19", "v=18", 1)},
		{"missing hash", validHash[:strings.LastIndex(validHash, "$")]},
		{"zero memory", strings.Replace(validHash, "m=19456", "m=0", 1)},
		{"zero iterations", strings.Replace(validHash, "t=2", "t=0", 1)},
		{"zero parallelism", strings.Replace(validHash, "p=1", "p=0", 1)},
		{"invalid salt", "$argon2id$v=19$m=19456,t=2,p=1$!!!$aGFzaA"},
		{"invalid hash", "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$!!!"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			valid, err := VerifyPassword(test.encode, passwordTestPassword)
			if err != nil {
				t.Fatalf("verify password: %v", err)
			}
			if valid {
				t.Fatal("malformed hash verified, want false")
			}
		})
	}
}

func TestHashPasswordRejectsInvalidPasswords(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"empty", ""},
		{"blank", "            "},
		{"too short", "short"},
		{"one below minimum", strings.Repeat("a", minPasswordLength-1)},
		{"one above maximum", strings.Repeat("a", maxPasswordLength+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := HashPassword(test.password); !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("error = %v, want ErrInvalidPassword", err)
			}
		})
	}
}

func TestVerifyPasswordRejectsInvalidPasswords(t *testing.T) {
	encoded, err := HashPassword(passwordTestPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
	}{
		{"empty", ""},
		{"blank", "            "},
		{"too short", "short"},
		{"one below minimum", strings.Repeat("a", minPasswordLength-1)},
		{"one above maximum", strings.Repeat("a", maxPasswordLength+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			valid, err := VerifyPassword(encoded, test.password)
			if err != nil {
				t.Fatalf("verify password: %v", err)
			}
			if valid {
				t.Fatal("invalid password verified, want false")
			}
		})
	}
}
