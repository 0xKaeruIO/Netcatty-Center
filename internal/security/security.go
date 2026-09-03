package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptN      = 16384
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 64
)

func RandomID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func RandomToken() string {
	var buf [32]byte
	_, _ = rand.Read(buf[:])
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// HashPassword matches the previous Node scrypt$salt$hash format so existing
// SQLite admin rows keep working. The salt string is used as UTF-8 bytes,
// same as crypto.scryptSync(password, saltHexString, 64).
func HashPassword(password string) (string, error) {
	var saltBytes [16]byte
	if _, err := rand.Read(saltBytes[:]); err != nil {
		return "", err
	}
	salt := hex.EncodeToString(saltBytes[:])
	hash, err := scrypt.Key([]byte(password), []byte(salt), scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", err
	}
	return "scrypt$" + salt + "$" + hex.EncodeToString(hash), nil
}

func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 3 || parts[0] != "scrypt" || parts[1] == "" || parts[2] == "" {
		return false
	}
	actual, err := scrypt.Key([]byte(password), []byte(parts[1]), scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[2])
	if err != nil || len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

type APIKey struct {
	Plaintext string
	Prefix    string
	Hash      string
}

func GenerateAPIKey() APIKey {
	var secret [24]byte
	_, _ = rand.Read(secret[:])
	plaintext := "ncc_" + hex.EncodeToString(secret[:])
	return APIKey{
		Plaintext: plaintext,
		Prefix:    plaintext[:12],
		Hash:      SHA256Hex(plaintext),
	}
}

func ValidAPIKeyFormat(plaintext string) bool {
	return strings.HasPrefix(plaintext, "ncc_") && len(plaintext) > 4
}

func MustHashPassword(password string) string {
	hash, err := HashPassword(password)
	if err != nil {
		panic(fmt.Errorf("hash password: %w", err))
	}
	return hash
}
