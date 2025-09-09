package auth

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "testpassword"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash should not be empty")
	}

	// Test verification with correct password
	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}

	if !valid {
		t.Fatal("Password verification should succeed")
	}

	// Test verification with wrong password
	valid, err = VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("Failed to verify wrong password: %v", err)
	}

	if valid {
		t.Fatal("Wrong password verification should fail")
	}
}

func TestJWTTokens(t *testing.T) {
	secret := "test-secret"
	userID := "user-123"
	username := "testuser"
	role := "org_user"
	orgID := "org-456"

	token, err := GenerateToken(userID, username, role, orgID, secret)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("Token should not be empty")
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}

	if claims.Username != username {
		t.Errorf("Expected Username %s, got %s", username, claims.Username)
	}

	if claims.Role != role {
		t.Errorf("Expected Role %s, got %s", role, claims.Role)
	}

	if claims.OrgID != orgID {
		t.Errorf("Expected OrgID %s, got %s", orgID, claims.OrgID)
	}
}
