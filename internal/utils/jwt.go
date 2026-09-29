package utils

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	UserRole  string `json:"user_role"`
	UserEmail string `json:"user_email"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}
type OauthClaims struct {
	Provider_id string `json:"provider_id"`
	Provider    string `json:"provider"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
}

// TokenService is the narrow seam that the rest of the app depends on instead of
// the concrete JWT implementation.
//
// Handlers and middleware only ever need two things: "issue a token for this
// user" and "tell me who this token belongs to". Depending on this interface
// rather than on concrete functions keeps the signing library, the key storage
// and the clock out of their code, and lets tests inject a fake that returns
// canned claims without a real secret.
type TokenService interface {
	GenerateAccessToken(userID uint, username, userRole, userEmail string) (string, error)
	GenerateOAuthToken(providerID, provider string) (string, error)
	VerifyAccessToken(token string) (*Claims, error)
	VerifyOAuthToken(token string) (*OauthClaims, error)
}

// JWTSigner is the production TokenService.
//
// The signing secret is a constructor argument instead of being read from the
// global config.AppConfig inside every call. That makes the secret an explicit,
// visible dependency: a test (or a differently-configured environment) can hand
// the signer its own key, and nothing outside this type needs to know where the
// secret came from. THIS is the dependency-injection point for tokens.
type JWTSigner struct {
	secret []byte
}

// NewJWTSigner wires a signer to a specific secret.
func NewJWTSigner(secret string) *JWTSigner {
	return &JWTSigner{secret: []byte(secret)}
}

func (s *JWTSigner) GenerateAccessToken(userID uint, username string, userRole string, userEmail string) (string, error) {
	now := time.Now()
	return sign(s.secret, jwt.MapClaims{
		"user_id":    userID,
		"username":   username,
		"user_role":  userRole,
		"user_email": userEmail,
		"iat":        now.Unix(),
		"exp":        now.Add(24 * time.Hour).Unix(),
	})
}

func (s *JWTSigner) GenerateOAuthToken(providerID string, provider string) (string, error) {
	now := time.Now()
	return sign(s.secret, jwt.MapClaims{
		"provider_id": providerID,
		"provider":    provider,
		"iat":         now.Unix(),
		"exp":         now.Add(24 * time.Hour).Unix(),
	})
}

func (s *JWTSigner) VerifyAccessToken(token string) (*Claims, error) {
	return verify[Claims](s.secret, token)
}

func (s *JWTSigner) VerifyOAuthToken(token string) (*OauthClaims, error) {
	return verify[OauthClaims](s.secret, token)
}

func sign(secret []byte, claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func verify[T any](secret []byte, tokenString string) (*T, error) {
	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("Unexpected signing method")
			}

			return secret, nil
		},
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("Could not parse claims")
	}

	claimBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	var customClaims T
	if err := json.Unmarshal(claimBytes, &customClaims); err != nil {
		return nil, err
	}

	return &customClaims, nil
}
