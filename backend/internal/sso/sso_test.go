package sso

import (
	"errors"
	"testing"
	"time"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	payload := &Payload{
		Type:             "sftp",
		Host:             "example.com",
		Port:             22,
		Username:         "testuser",
		Password:         "testpass",
		InitialDirectory: "/home/testuser",
		Language:         "en",
		Exp:              time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := Encrypt(payload, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if token == "" {
		t.Fatal("Encrypt returned empty token")
	}

	decrypted, err := Decrypt(token, secret)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted.Type != payload.Type {
		t.Errorf("Type mismatch: got %s, want %s", decrypted.Type, payload.Type)
	}
	if decrypted.Host != payload.Host {
		t.Errorf("Host mismatch: got %s, want %s", decrypted.Host, payload.Host)
	}
	if decrypted.Port != payload.Port {
		t.Errorf("Port mismatch: got %d, want %d", decrypted.Port, payload.Port)
	}
	if decrypted.Username != payload.Username {
		t.Errorf("Username mismatch: got %s, want %s", decrypted.Username, payload.Username)
	}
	if decrypted.Password != payload.Password {
		t.Errorf("Password mismatch: got %s, want %s", decrypted.Password, payload.Password)
	}
	if decrypted.InitialDirectory != payload.InitialDirectory {
		t.Errorf("InitialDirectory mismatch: got %s, want %s", decrypted.InitialDirectory, payload.InitialDirectory)
	}
	if decrypted.Language != payload.Language {
		t.Errorf("Language mismatch: got %s, want %s", decrypted.Language, payload.Language)
	}
	if decrypted.Exp != payload.Exp {
		t.Errorf("Exp mismatch: got %d, want %d", decrypted.Exp, payload.Exp)
	}
}

func TestDecryptExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	payload := &Payload{
		Type:     "sftp",
		Host:     "example.com",
		Port:     22,
		Username: "testuser",
		Password: "testpass",
		Exp:      time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
	}

	token, err := Encrypt(payload, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(token, secret)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("Expected ErrTokenExpired, got: %v", err)
	}
}

func TestDecryptWrongSecret(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	wrongSecret := []byte("wrong-secret-key-minimum-32bytes!")
	payload := &Payload{
		Type:     "sftp",
		Host:     "example.com",
		Port:     22,
		Username: "testuser",
		Password: "testpass",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := Encrypt(payload, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(token, wrongSecret)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("Expected ErrTokenInvalid, got: %v", err)
	}
}

func TestDecryptTamperedToken(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	payload := &Payload{
		Type:     "sftp",
		Host:     "example.com",
		Port:     22,
		Username: "testuser",
		Password: "testpass",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := Encrypt(payload, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the token by flipping a byte
	tokenBytes := []byte(token)
	if len(tokenBytes) > 10 {
		tokenBytes[10] ^= 1 // flip one bit
	}
	tamperedToken := string(tokenBytes)

	_, err = Decrypt(tamperedToken, secret)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("Expected ErrTokenInvalid for tampered token, got: %v", err)
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	invalidToken := "this is not valid base64!!!"

	_, err := Decrypt(invalidToken, secret)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("Expected ErrTokenInvalid for invalid base64, got: %v", err)
	}
}

func TestDecryptTokenTooShort(t *testing.T) {
	secret := []byte("test-secret-key-minimum-32-bytes!")
	// Create a token that's too short (less than 29 bytes when decoded)
	shortToken := "YWJjZGVmZ2hpamts" // "abcdefghijkl" in base64, only 12 bytes

	_, err := Decrypt(shortToken, secret)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("Expected ErrTokenInvalid for token too short, got: %v", err)
	}
}

func TestUsedSetMarkAndIsUsed(t *testing.T) {
	us := NewUsedSet()
	defer us.Stop()

	tokenHash := "test-token-hash-123"
	expiry := time.Now().Add(1 * time.Hour)

	// Initially should not be used
	if us.IsUsed(tokenHash) {
		t.Error("Token should not be marked as used initially")
	}

	// Mark the token
	us.Mark(tokenHash, expiry)

	// Now it should be used
	if !us.IsUsed(tokenHash) {
		t.Error("Token should be marked as used after Mark")
	}

	// Check another token that wasn't marked
	if us.IsUsed("different-token") {
		t.Error("Different token should not be marked as used")
	}
}

func TestUsedSetExpiredEntryNotUsed(t *testing.T) {
	us := NewUsedSet()
	defer us.Stop()

	tokenHash := "expired-token-hash"
	expiry := time.Now().Add(-1 * time.Second) // already expired

	// Mark with expired time
	us.Mark(tokenHash, expiry)

	// Should return false for expired token
	if us.IsUsed(tokenHash) {
		t.Error("Expired token should not be considered used")
	}
}
