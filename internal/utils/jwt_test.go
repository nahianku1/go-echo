package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAccessTokenUsesShortLifetime(t *testing.T) {
	signer := NewJWTSigner("test-secret")
	raw, err := signer.GenerateAccessToken(1, "user", "user", "user@example.com")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	token, _, err := new(jwt.Parser).ParseUnverified(raw, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected JWT map claims")
	}

	iat, ok := claims["iat"].(float64)
	if !ok {
		t.Fatal("expected issued-at claim")
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("expected expiry claim")
	}
	if time.Duration(exp-iat)*time.Second != AccessTokenLifetime {
		t.Fatalf("expected access-token lifetime %s, got %s", AccessTokenLifetime, time.Duration(exp-iat)*time.Second)
	}
}
