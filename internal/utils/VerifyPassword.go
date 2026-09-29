package utils

import (
	"github.com/alexedwards/argon2id"
)

// VerifyPassword checks if the provided plain password matches the hashed password
func VerifyPassword(pass string, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(pass, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}
