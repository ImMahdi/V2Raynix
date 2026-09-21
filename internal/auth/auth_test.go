package auth_test

import (
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/auth"
)

func TestAuth_PasswordHashing(t *testing.T) {
	rawPassword := "SuperSecretAdmin123!"

	hash, err := auth.HashPassword(rawPassword)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == "" || hash == rawPassword {
		t.Fatalf("invalid hash output")
	}

	// Correct password must match
	if !auth.CheckPassword(hash, rawPassword) {
		t.Errorf("expected password to match hash")
	}

	// Wrong password must fail
	if auth.CheckPassword(hash, "WrongPassword") {
		t.Errorf("wrong password should not match hash")
	}
}

func TestAuth_JWTTokens(t *testing.T) {
	secret := []byte("v2raynix-super-secret-jwt-key-256bit")
	username := "admin"

	// 1. Generate token
	token, err := auth.GenerateJWT(username, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}
	if token == "" {
		t.Fatalf("empty token generated")
	}

	// 2. Validate valid token
	claims, err := auth.ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.Username != username {
		t.Errorf("expected username %s, got %s", username, claims.Username)
	}

	// 3. Reject token with wrong secret
	_, err = auth.ValidateJWT(token, []byte("wrong-secret-key-at-least-32-bytes-long!"))
	if err == nil {
		t.Errorf("expected error when validating with wrong secret")
	}

	// 4. Test expired token
	expiredToken, err := auth.GenerateJWT(username, secret, -1*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	_, err = auth.ValidateJWT(expiredToken, secret)
	if err == nil {
		t.Errorf("expected error validating expired token")
	}
}

func TestWeakSecretValidation(t *testing.T) {
	// 1. Empty secret in GenerateJWT
	_, err := auth.GenerateJWT("admin", []byte(""), 1*time.Hour)
	if err != auth.ErrWeakSecret {
		t.Errorf("expected ErrWeakSecret for empty key, got: %v", err)
	}

	// 2. Short secret (< 32 bytes) in GenerateJWT
	_, err = auth.GenerateJWT("admin", []byte("short-secret-key-16-bytes!"), 1*time.Hour)
	if err != auth.ErrWeakSecret {
		t.Errorf("expected ErrWeakSecret for short key, got: %v", err)
	}

	// 3. Empty secret in ValidateJWT
	validSecret := []byte("v2raynix-super-secret-jwt-key-256bit")
	token, _ := auth.GenerateJWT("admin", validSecret, 1*time.Hour)

	_, err = auth.ValidateJWT(token, []byte(""))
	if err != auth.ErrWeakSecret {
		t.Errorf("expected ErrWeakSecret for empty key in ValidateJWT, got: %v", err)
	}

	// 4. Short secret in ValidateJWT
	_, err = auth.ValidateJWT(token, []byte("short-secret-key-16-bytes!"))
	if err != auth.ErrWeakSecret {
		t.Errorf("expected ErrWeakSecret for short key in ValidateJWT, got: %v", err)
	}
}
