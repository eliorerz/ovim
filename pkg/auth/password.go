package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2 configuration constants
	DefaultArgon2Time    = 1
	DefaultArgon2Memory  = 64 * 1024
	DefaultArgon2Threads = 4
	DefaultArgon2KeyLen  = 32
	SaltLength           = 16

	// Password format constants
	PasswordHashPrefix = "$argon2id$"
)

var (
	ErrInvalidHashFormat   = errors.New("invalid hash format")
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong     = errors.New("password must be less than 128 characters")
)

// PasswordConfig holds Argon2 configuration parameters
type PasswordConfig struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

// DefaultPasswordConfig returns the default password hashing configuration
func DefaultPasswordConfig() *PasswordConfig {
	return &PasswordConfig{
		Time:    DefaultArgon2Time,
		Memory:  DefaultArgon2Memory,
		Threads: DefaultArgon2Threads,
		KeyLen:  DefaultArgon2KeyLen,
	}
}

// PasswordHasher handles password hashing and verification
type PasswordHasher struct {
	config *PasswordConfig
}

// NewPasswordHasher creates a new password hasher with the given config
func NewPasswordHasher(config *PasswordConfig) *PasswordHasher {
	if config == nil {
		config = DefaultPasswordConfig()
	}
	return &PasswordHasher{config: config}
}

// HashPassword hashes a password using Argon2id
func (ph *PasswordHasher) HashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, ph.config.Time, ph.config.Memory, ph.config.Threads, ph.config.KeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		PasswordHashPrefix, argon2.Version, ph.config.Memory, ph.config.Time, ph.config.Threads, b64Salt, b64Hash)

	return encodedHash, nil
}

// VerifyPassword verifies a password against its hash
func (ph *PasswordHasher) VerifyPassword(password, encodedHash string) (bool, error) {
	if err := validatePassword(password); err != nil {
		return false, err
	}

	if !strings.HasPrefix(encodedHash, PasswordHashPrefix) {
		return false, ErrInvalidHashFormat
	}

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, ErrInvalidHashFormat
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("failed to parse version: %w", err)
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("failed to parse parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	otherHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(hash)))

	return subtle.ConstantTimeCompare(hash, otherHash) == 1, nil
}

// validatePassword performs basic password validation
func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 128 {
		return ErrPasswordTooLong
	}
	return nil
}

// Default global password hasher for convenience
var defaultHasher = NewPasswordHasher(nil)

// HashPassword hashes a password using the default configuration
func HashPassword(password string) (string, error) {
	return defaultHasher.HashPassword(password)
}

// VerifyPassword verifies a password using the default configuration
func VerifyPassword(password, encodedHash string) (bool, error) {
	return defaultHasher.VerifyPassword(password, encodedHash)
}
