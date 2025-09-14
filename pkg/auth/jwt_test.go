package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenManager(t *testing.T) {
	t.Run("WithDuration", func(t *testing.T) {
		duration := 2 * time.Hour
		tm := NewTokenManager("secret", duration)
		assert.Equal(t, duration, tm.duration)
		assert.Equal(t, []byte("secret"), tm.secret)
	})

	t.Run("WithZeroDuration", func(t *testing.T) {
		tm := NewTokenManager("secret", 0)
		assert.Equal(t, DefaultTokenDuration, tm.duration)
	})
}

func TestTokenManager_GenerateToken(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)

	t.Run("ValidTokenGeneration", func(t *testing.T) {
		token, err := tm.GenerateToken("user-123", "testuser", "user", "org-456")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Verify token can be parsed
		claims := &Claims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte("test-secret"), nil
		})
		require.NoError(t, err)
		assert.True(t, parsedToken.Valid)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "user", claims.Role)
		assert.Equal(t, "org-456", claims.OrgID)
		assert.Equal(t, JWTIssuer, claims.Issuer)
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		_, err := tm.GenerateToken("", "testuser", "user", "org-456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "userID, username, and role are required")
	})

	t.Run("EmptyUsername", func(t *testing.T) {
		_, err := tm.GenerateToken("user-123", "", "user", "org-456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "userID, username, and role are required")
	})

	t.Run("EmptyRole", func(t *testing.T) {
		_, err := tm.GenerateToken("user-123", "testuser", "", "org-456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "userID, username, and role are required")
	})

	t.Run("EmptyOrgID", func(t *testing.T) {
		token, err := tm.GenerateToken("user-123", "testuser", "system_admin", "")
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func TestTokenManager_ValidateToken(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)

	t.Run("ValidToken", func(t *testing.T) {
		// Generate a token first
		originalToken, err := tm.GenerateToken("user-123", "testuser", "user", "org-456")
		require.NoError(t, err)

		// Validate it
		claims, err := tm.ValidateToken(originalToken)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "user", claims.Role)
		assert.Equal(t, "org-456", claims.OrgID)
	})

	t.Run("EmptyToken", func(t *testing.T) {
		_, err := tm.ValidateToken("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token string cannot be empty")
	})

	t.Run("InvalidToken", func(t *testing.T) {
		_, err := tm.ValidateToken("invalid.token.string")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")
	})

	t.Run("WrongSecret", func(t *testing.T) {
		wrongTM := NewTokenManager("wrong-secret", time.Hour)
		originalToken, err := tm.GenerateToken("user-123", "testuser", "user", "org-456")
		require.NoError(t, err)

		_, err = wrongTM.ValidateToken(originalToken)
		assert.Error(t, err)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		shortTM := NewTokenManager("test-secret", -time.Hour) // Already expired
		expiredToken, err := shortTM.GenerateToken("user-123", "testuser", "user", "org-456")
		require.NoError(t, err)

		_, err = shortTM.ValidateToken(expiredToken)
		assert.Error(t, err)
	})

	t.Run("InvalidClaims", func(t *testing.T) {
		// Create token with invalid claims manually
		claims := &Claims{
			UserID:   "", // Invalid empty userID
			Username: "testuser",
			Role:     "user",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Issuer:    JWTIssuer,
				Subject:   "user-123",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("test-secret"))
		require.NoError(t, err)

		_, err = tm.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token contains invalid claims")
	})

	t.Run("WrongSigningMethod", func(t *testing.T) {
		// Test the validation path with a manually created invalid token
		invalidToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature"
		_, err := tm.ValidateToken(invalidToken)
		assert.Error(t, err)
	})
}

func TestLegacyFunctions(t *testing.T) {
	secret := "legacy-secret"

	t.Run("LegacyGenerateToken", func(t *testing.T) {
		token, err := GenerateToken("user-123", "testuser", "user", "org-456", secret)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("LegacyValidateToken", func(t *testing.T) {
		token, err := GenerateToken("user-123", "testuser", "user", "org-456", secret)
		require.NoError(t, err)

		claims, err := ValidateToken(token, secret)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "user", claims.Role)
		assert.Equal(t, "org-456", claims.OrgID)
	})

	t.Run("LegacyTokenRoundTrip", func(t *testing.T) {
		// Test that legacy functions work together
		originalUserID := "legacy-user"
		originalUsername := "legacyuser"
		originalRole := "admin"
		originalOrgID := "legacy-org"

		token, err := GenerateToken(originalUserID, originalUsername, originalRole, originalOrgID, secret)
		require.NoError(t, err)

		claims, err := ValidateToken(token, secret)
		require.NoError(t, err)

		assert.Equal(t, originalUserID, claims.UserID)
		assert.Equal(t, originalUsername, claims.Username)
		assert.Equal(t, originalRole, claims.Role)
		assert.Equal(t, originalOrgID, claims.OrgID)
	})
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 24*time.Hour, DefaultTokenDuration)
	assert.Equal(t, "ovim-backend", JWTIssuer)
	assert.Equal(t, "HS256", JWTSigningMethod)
}

func TestClaims(t *testing.T) {
	claims := &Claims{
		UserID:   "test-user",
		Username: "testuser",
		Role:     "admin",
		OrgID:    "test-org",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  JWTIssuer,
			Subject: "test-user",
		},
	}

	assert.Equal(t, "test-user", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "test-org", claims.OrgID)
	assert.Equal(t, JWTIssuer, claims.Issuer)
	assert.Equal(t, "test-user", claims.Subject)
}

func TestTokenManagerConcurrency(t *testing.T) {
	tm := NewTokenManager("concurrent-secret", time.Hour)

	// Test concurrent token generation
	tokenChan := make(chan string, 10)
	errorChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			token, err := tm.GenerateToken(
				fmt.Sprintf("user-%d", id),
				fmt.Sprintf("user%d", id),
				"user",
				fmt.Sprintf("org-%d", id%3),
			)
			if err != nil {
				errorChan <- err
				return
			}
			tokenChan <- token
		}(i)
	}

	// Collect results
	tokens := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		select {
		case token := <-tokenChan:
			tokens = append(tokens, token)
		case err := <-errorChan:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for token generation")
		}
	}

	assert.Len(t, tokens, 10)

	// Verify all tokens are valid and unique
	seenTokens := make(map[string]bool)
	for _, token := range tokens {
		assert.False(t, seenTokens[token], "Duplicate token generated")
		seenTokens[token] = true

		claims, err := tm.ValidateToken(token)
		assert.NoError(t, err)
		assert.NotEmpty(t, claims.UserID)
	}
}
