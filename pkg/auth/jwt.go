package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// JWT configuration constants
	DefaultTokenDuration = 24 * time.Hour
	JWTIssuer            = "ovim-backend"
	JWTSigningMethod     = "HS256"
)

// Claims represents JWT claims for OVIM
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	OrgID    string `json:"org_id,omitempty"`
	jwt.RegisteredClaims
}

// TokenManager handles JWT token operations
type TokenManager struct {
	secret   []byte
	duration time.Duration
}

// NewTokenManager creates a new token manager
func NewTokenManager(secret string, duration time.Duration) *TokenManager {
	if duration == 0 {
		duration = DefaultTokenDuration
	}
	return &TokenManager{
		secret:   []byte(secret),
		duration: duration,
	}
}

// GenerateToken creates a new JWT token for the user
func (tm *TokenManager) GenerateToken(userID, username, role, orgID string) (string, error) {
	if userID == "" || username == "" || role == "" {
		return "", fmt.Errorf("userID, username, and role are required")
	}

	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		OrgID:    orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    JWTIssuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

// ValidateToken validates a JWT token and returns the claims
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token string cannot be empty")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	// Additional validation
	if claims.UserID == "" || claims.Username == "" || claims.Role == "" {
		return nil, fmt.Errorf("token contains invalid claims")
	}

	return claims, nil
}

// Legacy functions for backward compatibility
func GenerateToken(userID, username, role, orgID, secret string) (string, error) {
	tm := NewTokenManager(secret, DefaultTokenDuration)
	return tm.GenerateToken(userID, username, role, orgID)
}

func ValidateToken(tokenString, secret string) (*Claims, error) {
	tm := NewTokenManager(secret, DefaultTokenDuration)
	return tm.ValidateToken(tokenString)
}
