package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

func GenerateHexOTP() (string, error) {
	b := make([]byte, 3)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(b)), nil
}
