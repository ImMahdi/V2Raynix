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
	_, err = auth.ValidateJWT(token, []byte("wrong-secret-key-1234567890"))
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
