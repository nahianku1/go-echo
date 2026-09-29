package utils

import (
	"github.com/alexedwards/argon2id"
)

// CreatePasswordHash generates an Argon2id hash for the provided password
func CreatePasswordHash(pass string) string {
	logger := NewLogger("HASH")
	hash, err := argon2id.CreateHash(pass, argon2id.DefaultParams)
	if err != nil {
		logger.Printf("Failed creating password hash: %v", err)
	}
	return hash
}
