package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordHasher(t *testing.T) {
	t.Run("WithConfig", func(t *testing.T) {
		config := &PasswordConfig{
			Time:    2,
			Memory:  128 * 1024,
			Threads: 8,
			KeyLen:  64,
		}
		hasher := NewPasswordHasher(config)
		assert.Equal(t, config, hasher.config)
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		hasher := NewPasswordHasher(nil)
		assert.Equal(t, DefaultPasswordConfig(), hasher.config)
	})
}

func TestDefaultPasswordConfig(t *testing.T) {
	config := DefaultPasswordConfig()
	assert.Equal(t, uint32(DefaultArgon2Time), config.Time)
	assert.Equal(t, uint32(DefaultArgon2Memory), config.Memory)
	assert.Equal(t, uint8(DefaultArgon2Threads), config.Threads)
	assert.Equal(t, uint32(DefaultArgon2KeyLen), config.KeyLen)
}

func TestPasswordHasher_HashPassword(t *testing.T) {
	hasher := NewPasswordHasher(nil)

	t.Run("ValidPassword", func(t *testing.T) {
		password := "validpassword123"
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, PasswordHashPrefix)
		assert.Contains(t, hash, "v=")
		assert.Contains(t, hash, "m=")
		assert.Contains(t, hash, "t=")
		assert.Contains(t, hash, "p=")
	})

	t.Run("PasswordTooShort", func(t *testing.T) {
		password := "short"
		_, err := hasher.HashPassword(password)
		assert.Error(t, err)
		assert.Equal(t, ErrPasswordTooShort, err)
	})

	t.Run("PasswordTooLong", func(t *testing.T) {
		password := strings.Repeat("a", 129)
		_, err := hasher.HashPassword(password)
		assert.Error(t, err)
		assert.Equal(t, ErrPasswordTooLong, err)
	})

	t.Run("MinimumValidPassword", func(t *testing.T) {
		password := "12345678" // Exactly 8 characters
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("MaximumValidPassword", func(t *testing.T) {
		password := strings.Repeat("a", 128) // Exactly 128 characters
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("UniqueHashes", func(t *testing.T) {
		password := "samepassword123"
		hash1, err1 := hasher.HashPassword(password)
		hash2, err2 := hasher.HashPassword(password)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Same password should generate different hashes due to salt")
	})
}

func TestPasswordHasher_VerifyPassword(t *testing.T) {
	hasher := NewPasswordHasher(nil)

	t.Run("CorrectPassword", func(t *testing.T) {
		password := "correctpassword123"
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)

		valid, err := hasher.VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("IncorrectPassword", func(t *testing.T) {
		password := "correctpassword123"
		wrongPassword := "wrongpassword123"
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)

		valid, err := hasher.VerifyPassword(wrongPassword, hash)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("InvalidPasswordTooShort", func(t *testing.T) {
		hash := "$argon2id$v=19$m=65536,t=1,p=4$dGVzdA$hash"
		_, err := hasher.VerifyPassword("short", hash)
		assert.Error(t, err)
		assert.Equal(t, ErrPasswordTooShort, err)
	})

	t.Run("InvalidPasswordTooLong", func(t *testing.T) {
		hash := "$argon2id$v=19$m=65536,t=1,p=4$dGVzdA$hash"
		longPassword := strings.Repeat("a", 129)
		_, err := hasher.VerifyPassword(longPassword, hash)
		assert.Error(t, err)
		assert.Equal(t, ErrPasswordTooLong, err)
	})

	t.Run("InvalidHashFormat", func(t *testing.T) {
		password := "validpassword123"
		invalidHash := "invalid-hash-format"
		_, err := hasher.VerifyPassword(password, invalidHash)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHashFormat, err)
	})

	t.Run("WrongPrefix", func(t *testing.T) {
		password := "validpassword123"
		wrongPrefixHash := "$bcrypt$wrongformat"
		_, err := hasher.VerifyPassword(password, wrongPrefixHash)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHashFormat, err)
	})

	t.Run("IncorrectNumberOfParts", func(t *testing.T) {
		password := "validpassword123"
		malformedHash := "$argon2id$v=19$m=65536"
		_, err := hasher.VerifyPassword(password, malformedHash)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHashFormat, err)
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		password := "validpassword123"
		invalidVersionHash := "$argon2id$v=invalid$m=65536,t=1,p=4$dGVzdA$hash"
		_, err := hasher.VerifyPassword(password, invalidVersionHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse version")
	})

	t.Run("IncompatibleVersion", func(t *testing.T) {
		password := "validpassword123"
		incompatibleVersionHash := "$argon2id$v=18$m=65536,t=1,p=4$dGVzdA$hash"
		_, err := hasher.VerifyPassword(password, incompatibleVersionHash)
		assert.Error(t, err)
		assert.Equal(t, ErrIncompatibleVersion, err)
	})

	t.Run("InvalidParameters", func(t *testing.T) {
		password := "validpassword123"
		invalidParamsHash := "$argon2id$v=19$invalid-params$dGVzdA$hash"
		_, err := hasher.VerifyPassword(password, invalidParamsHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse parameters")
	})

	t.Run("InvalidSalt", func(t *testing.T) {
		password := "validpassword123"
		invalidSaltHash := "$argon2id$v=19$m=65536,t=1,p=4$invalid-base64!$hash"
		_, err := hasher.VerifyPassword(password, invalidSaltHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode salt")
	})

	t.Run("InvalidHash", func(t *testing.T) {
		password := "validpassword123"
		invalidHashHash := "$argon2id$v=19$m=65536,t=1,p=4$dGVzdA$invalid-base64!"
		_, err := hasher.VerifyPassword(password, invalidHashHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode hash")
	})
}

func TestPasswordRoundTrip(t *testing.T) {
	hasher := NewPasswordHasher(nil)
	testPasswords := []string{
		"simplepassword",
		"P@ssw0rd123!",
		"very-long-password-with-special-chars-áéíóú-123456789",
		"12345678",               // minimum length
		strings.Repeat("a", 128), // maximum length
	}

	for _, password := range testPasswords {
		t.Run("Password_"+password[:min(len(password), 20)], func(t *testing.T) {
			// Hash the password
			hash, err := hasher.HashPassword(password)
			require.NoError(t, err)

			// Verify the correct password
			valid, err := hasher.VerifyPassword(password, hash)
			require.NoError(t, err)
			assert.True(t, valid)

			// Verify an incorrect password
			wrongPassword := password + "wrong"
			if len(wrongPassword) <= 128 {
				valid, err = hasher.VerifyPassword(wrongPassword, hash)
				require.NoError(t, err)
				assert.False(t, valid)
			}
		})
	}
}

func TestPasswordHasherWithCustomConfig(t *testing.T) {
	config := &PasswordConfig{
		Time:    2,
		Memory:  32 * 1024,
		Threads: 2,
		KeyLen:  16,
	}
	hasher := NewPasswordHasher(config)

	password := "testpassword123"
	hash, err := hasher.HashPassword(password)
	require.NoError(t, err)

	// Verify the hash contains the custom parameters
	assert.Contains(t, hash, "m=32768") // 32 * 1024
	assert.Contains(t, hash, "t=2")
	assert.Contains(t, hash, "p=2")

	// Verify the password
	valid, err := hasher.VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestLegacyPasswordFunctions(t *testing.T) {
	t.Run("HashPassword", func(t *testing.T) {
		password := "legacypassword123"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, PasswordHashPrefix)
	})

	t.Run("VerifyPassword", func(t *testing.T) {
		password := "legacypassword123"
		hash, err := HashPassword(password)
		require.NoError(t, err)

		valid, err := VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = VerifyPassword("wrongpassword", hash)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("LegacyRoundTrip", func(t *testing.T) {
		password := "roundtrippassword123"

		hash, err := HashPassword(password)
		require.NoError(t, err)

		valid, err := VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, valid)
	})
}

func TestValidatePassword(t *testing.T) {
	t.Run("ValidPassword", func(t *testing.T) {
		err := validatePassword("validpassword")
		assert.NoError(t, err)
	})

	t.Run("TooShort", func(t *testing.T) {
		err := validatePassword("short")
		assert.Equal(t, ErrPasswordTooShort, err)
	})

	t.Run("TooLong", func(t *testing.T) {
		longPassword := strings.Repeat("a", 129)
		err := validatePassword(longPassword)
		assert.Equal(t, ErrPasswordTooLong, err)
	})

	t.Run("ExactlyMinLength", func(t *testing.T) {
		err := validatePassword("12345678")
		assert.NoError(t, err)
	})

	t.Run("ExactlyMaxLength", func(t *testing.T) {
		err := validatePassword(strings.Repeat("a", 128))
		assert.NoError(t, err)
	})
}

func TestPasswordConstants(t *testing.T) {
	assert.Equal(t, uint32(1), DefaultArgon2Time)
	assert.Equal(t, uint32(64*1024), DefaultArgon2Memory)
	assert.Equal(t, uint8(4), DefaultArgon2Threads)
	assert.Equal(t, uint32(32), DefaultArgon2KeyLen)
	assert.Equal(t, 16, SaltLength)
	assert.Equal(t, "$argon2id$", PasswordHashPrefix)
}

func TestErrors(t *testing.T) {
	assert.Error(t, ErrInvalidHashFormat)
	assert.Error(t, ErrIncompatibleVersion)
	assert.Error(t, ErrPasswordTooShort)
	assert.Error(t, ErrPasswordTooLong)

	assert.Contains(t, ErrInvalidHashFormat.Error(), "invalid hash format")
	assert.Contains(t, ErrIncompatibleVersion.Error(), "incompatible argon2 version")
	assert.Contains(t, ErrPasswordTooShort.Error(), "at least 8 characters")
	assert.Contains(t, ErrPasswordTooLong.Error(), "less than 128 characters")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
