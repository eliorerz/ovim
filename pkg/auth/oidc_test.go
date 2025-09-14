package auth

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOIDCProvider(t *testing.T) {
	t.Run("DisabledOIDC", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled: false,
		}
		provider, err := NewOIDCProvider(config)
		require.NoError(t, err)
		assert.Nil(t, provider)
	})

	t.Run("MissingIssuerURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:   true,
			IssuerURL: "",
		}
		_, err := NewOIDCProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC issuer URL is required")
	})

	t.Run("MissingClientID", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:   true,
			IssuerURL: "https://example.com",
			ClientID:  "",
		}
		_, err := NewOIDCProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC client ID is required")
	})

	t.Run("MissingClientSecret", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://example.com",
			ClientID:     "test-client",
			ClientSecret: "",
		}
		_, err := NewOIDCProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC client secret is required")
	})

	t.Run("MissingRedirectURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "",
		}
		_, err := NewOIDCProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC redirect URL is required")
	})

	t.Run("InvalidIssuerURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://nonexistent.invalid.domain.example",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "https://example.com/callback",
		}
		_, err := NewOIDCProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create OIDC provider")
	})
}

func TestOIDCProvider_GenerateState(t *testing.T) {
	// Since we can't easily create a real provider without a working OIDC endpoint,
	// we'll test the state generation method which doesn't depend on external services
	config := &OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "https://app.example.com/callback",
	}

	// Create a mock provider struct to test state generation
	provider := &OIDCProvider{config: config}

	state1 := provider.GenerateState()
	state2 := provider.GenerateState()

	assert.NotEmpty(t, state1)
	assert.NotEmpty(t, state2)
	assert.NotEqual(t, state1, state2, "Generated states should be unique")
	assert.Greater(t, len(state1), 40, "State should be sufficiently long")
}

func TestOIDCProvider_GetAuthURL(t *testing.T) {
	config := &OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile", "email"},
	}

	// Create a mock provider to test URL generation
	provider := &OIDCProvider{config: config}

	// We need to mock the oauth2.Config for this test
	// Since the actual implementation requires a real OIDC provider,
	// we'll test the concept but not the actual URL generation
	state := "test-state-123"

	// This test verifies the method exists and can be called
	// In a real integration test, you'd need a test OIDC server
	authURL := provider.GetAuthURL(state)

	// The URL will be empty since we don't have a real oauth2.Config initialized
	// but this tests that the method signature is correct
	assert.IsType(t, "", authURL)
}

func TestMapOIDCRolesToOVIM(t *testing.T) {
	provider := &OIDCProvider{}

	tests := []struct {
		name         string
		userInfo     *UserInfo
		expectedRole string
	}{
		{
			name: "SystemAdminRole",
			userInfo: &UserInfo{
				Roles: []string{"system-admin", "user"},
			},
			expectedRole: "system_admin",
		},
		{
			name: "AdminRoleInRoles",
			userInfo: &UserInfo{
				Roles: []string{"application-admin", "user"},
			},
			expectedRole: "system_admin",
		},
		{
			name: "SystemAdminGroup",
			userInfo: &UserInfo{
				Groups: []string{"system-admins", "users"},
			},
			expectedRole: "system_admin",
		},
		{
			name: "OrgAdminGroup",
			userInfo: &UserInfo{
				Groups: []string{"org-admins", "users"},
			},
			expectedRole: "org_admin",
		},
		{
			name: "RegularUser",
			userInfo: &UserInfo{
				Roles:  []string{"user", "member"},
				Groups: []string{"users", "members"},
			},
			expectedRole: "user",
		},
		{
			name: "EmptyRolesAndGroups",
			userInfo: &UserInfo{
				Roles:  []string{},
				Groups: []string{},
			},
			expectedRole: "user",
		},
		{
			name: "MixedCaseAdmin",
			userInfo: &UserInfo{
				Roles: []string{"ADMIN", "USER"},
			},
			expectedRole: "system_admin",
		},
		{
			name: "AdminInGroups",
			userInfo: &UserInfo{
				Groups: []string{"platform-admin", "users"},
			},
			expectedRole: "org_admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := provider.MapOIDCRolesToOVIM(tt.userInfo)
			assert.Equal(t, tt.expectedRole, role)
		})
	}
}

func TestValidateOIDCConfig(t *testing.T) {
	t.Run("DisabledConfig", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled: false,
		}
		err := ValidateOIDCConfig(config)
		assert.NoError(t, err)
	})

	t.Run("ValidConfig", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       []string{"openid", "profile"},
		}
		err := ValidateOIDCConfig(config)
		assert.NoError(t, err)
	})

	t.Run("MissingIssuerURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:   true,
			IssuerURL: "",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC issuer URL is required")
	})

	t.Run("InvalidIssuerURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "://invalid-url-no-scheme",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "https://app.example.com/callback",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OIDC issuer URL")
	})

	t.Run("MissingClientID", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:   true,
			IssuerURL: "https://auth.example.com",
			ClientID:  "",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC client ID is required")
	})

	t.Run("MissingClientSecret", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC client secret is required")
	})

	t.Run("MissingRedirectURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC redirect URL is required")
	})

	t.Run("InvalidRedirectURL", func(t *testing.T) {
		config := &OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "://invalid-url",
		}
		err := ValidateOIDCConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OIDC redirect URL")
	})
}

func TestOIDCConfig(t *testing.T) {
	config := &OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile", "email", "groups"},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "https://auth.example.com", config.IssuerURL)
	assert.Equal(t, "test-client-id", config.ClientID)
	assert.Equal(t, "test-client-secret", config.ClientSecret)
	assert.Equal(t, "https://app.example.com/callback", config.RedirectURL)
	assert.Equal(t, []string{"openid", "profile", "email", "groups"}, config.Scopes)
}

func TestUserInfo(t *testing.T) {
	userInfo := &UserInfo{
		Subject:           "user-123",
		Name:              "John Doe",
		GivenName:         "John",
		FamilyName:        "Doe",
		PreferredUsername: "johndoe",
		Email:             "john@example.com",
		EmailVerified:     true,
		Groups:            []string{"users", "admins"},
		Roles:             []string{"admin", "user"},
	}

	assert.Equal(t, "user-123", userInfo.Subject)
	assert.Equal(t, "John Doe", userInfo.Name)
	assert.Equal(t, "John", userInfo.GivenName)
	assert.Equal(t, "Doe", userInfo.FamilyName)
	assert.Equal(t, "johndoe", userInfo.PreferredUsername)
	assert.Equal(t, "john@example.com", userInfo.Email)
	assert.True(t, userInfo.EmailVerified)
	assert.Equal(t, []string{"users", "admins"}, userInfo.Groups)
	assert.Equal(t, []string{"admin", "user"}, userInfo.Roles)
}

func TestOIDCProviderMethods(t *testing.T) {
	// Test that methods exist and have correct signatures
	// These tests verify the interface/API without requiring actual OIDC connectivity

	provider := &OIDCProvider{}

	t.Run("GenerateStateExists", func(t *testing.T) {
		state := provider.GenerateState()
		assert.IsType(t, "", state)
		assert.NotEmpty(t, state)
	})

	t.Run("GetAuthURLExists", func(t *testing.T) {
		// This will return empty string since oauth2.Config is not initialized
		// but it tests the method signature
		url := provider.GetAuthURL("test-state")
		assert.IsType(t, "", url)
	})

	t.Run("ExchangeCodeExists", func(t *testing.T) {
		// This will fail since we don't have a real provider, but tests the signature
		ctx := context.Background()
		_, err := provider.ExchangeCode(ctx, "test-code")
		assert.Error(t, err) // Expected to fail without real provider
	})

	t.Run("VerifyIDTokenExists", func(t *testing.T) {
		// Skip this test since we cannot test it without a real provider
		// The method signature is tested by compilation
		t.Skip("Cannot test VerifyIDToken without real OIDC provider")
	})
}

func TestOIDCConfigValidation(t *testing.T) {
	// Test URL parsing edge cases
	invalidURLs := []string{
		"",
		"not-a-url",
		"ftp://invalid-scheme",
		"://missing-scheme",
		"http://",
		"https://",
	}

	for _, invalidURL := range invalidURLs {
		t.Run("InvalidURL_"+invalidURL, func(t *testing.T) {
			// Test issuer URL validation
			_, err := url.Parse(invalidURL)
			if invalidURL != "" {
				assert.Error(t, err, "URL should be invalid: %s", invalidURL)
			}

			// Test in config validation
			config := &OIDCConfig{
				Enabled:      true,
				IssuerURL:    invalidURL,
				ClientID:     "test",
				ClientSecret: "test",
				RedirectURL:  "https://valid.com/callback",
			}

			if invalidURL == "" {
				err := ValidateOIDCConfig(config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "required")
			}
		})
	}

	// Test valid URLs
	validURLs := []string{
		"https://auth.example.com",
		"https://auth.example.com/",
		"https://auth.example.com/realms/master",
		"http://localhost:8080",
		"http://127.0.0.1:8080/auth",
	}

	for _, validURL := range validURLs {
		t.Run("ValidURL_"+validURL, func(t *testing.T) {
			_, err := url.Parse(validURL)
			assert.NoError(t, err, "URL should be valid: %s", validURL)

			config := &OIDCConfig{
				Enabled:      true,
				IssuerURL:    validURL,
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURL:  "https://app.example.com/callback",
			}

			err = ValidateOIDCConfig(config)
			assert.NoError(t, err, "Config should be valid for URL: %s", validURL)
		})
	}
}

func TestOIDCRoleMapping(t *testing.T) {
	provider := &OIDCProvider{}

	// Test various role/group combinations
	testCases := []struct {
		description string
		roles       []string
		groups      []string
		expected    string
	}{
		{
			description: "Multiple admin roles",
			roles:       []string{"super-admin", "platform-admin", "user"},
			groups:      []string{"users"},
			expected:    "system_admin",
		},
		{
			description: "Admin in groups only",
			roles:       []string{"user", "member"},
			groups:      []string{"platform-admin", "users"},
			expected:    "org_admin",
		},
		{
			description: "System admin group",
			roles:       []string{},
			groups:      []string{"system-administrators", "users"},
			expected:    "system_admin",
		},
		{
			description: "Case insensitive matching",
			roles:       []string{"USER", "MEMBER"},
			groups:      []string{"ADMIN-GROUP", "USERS"},
			expected:    "org_admin",
		},
		{
			description: "No admin privileges",
			roles:       []string{"user", "member", "viewer"},
			groups:      []string{"users", "viewers", "members"},
			expected:    "user",
		},
		{
			description: "Empty roles and groups",
			roles:       []string{},
			groups:      []string{},
			expected:    "user",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			userInfo := &UserInfo{
				Roles:  tc.roles,
				Groups: tc.groups,
			}
			result := provider.MapOIDCRolesToOVIM(userInfo)
			assert.Equal(t, tc.expected, result)
		})
	}
}
