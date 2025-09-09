package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig holds OpenID Connect configuration
type OIDCConfig struct {
	Enabled      bool     `yaml:"enabled"`
	IssuerURL    string   `yaml:"issuerUrl"`
	ClientID     string   `yaml:"clientId"`
	ClientSecret string   `yaml:"clientSecret"`
	RedirectURL  string   `yaml:"redirectUrl"`
	Scopes       []string `yaml:"scopes"`
}

// OIDCProvider handles OpenID Connect authentication
type OIDCProvider struct {
	config   *OIDCConfig
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
	provider *oidc.Provider
}

// NewOIDCProvider creates a new OIDC provider
func NewOIDCProvider(config *OIDCConfig) (*OIDCProvider, error) {
	if !config.Enabled {
		return nil, nil
	}

	if config.IssuerURL == "" {
		return nil, fmt.Errorf("OIDC issuer URL is required")
	}

	if config.ClientID == "" {
		return nil, fmt.Errorf("OIDC client ID is required")
	}

	if config.ClientSecret == "" {
		return nil, fmt.Errorf("OIDC client secret is required")
	}

	if config.RedirectURL == "" {
		return nil, fmt.Errorf("OIDC redirect URL is required")
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Configure OIDC verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	// Set default scopes if none provided
	scopes := config.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	// Configure OAuth2
	oauth2Config := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &OIDCProvider{
		config:   config,
		verifier: verifier,
		oauth2:   oauth2Config,
		provider: provider,
	}, nil
}

// GetAuthURL generates an OAuth2 authorization URL
func (p *OIDCProvider) GetAuthURL(state string) string {
	return p.oauth2.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// GenerateState generates a random state parameter for OAuth2
func (p *OIDCProvider) GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// ExchangeCode exchanges an authorization code for tokens
func (p *OIDCProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// VerifyIDToken verifies and parses an ID token
func (p *OIDCProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}
	return idToken, nil
}

// UserInfo represents user information from OIDC
type UserInfo struct {
	Subject           string   `json:"sub"`
	Name              string   `json:"name"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Groups            []string `json:"groups"`
	Roles             []string `json:"realm_access,omitempty"`
}

// GetUserInfo extracts user information from ID token
func (p *OIDCProvider) GetUserInfo(ctx context.Context, idToken *oidc.IDToken) (*UserInfo, error) {
	var userInfo UserInfo
	if err := idToken.Claims(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to extract user info: %w", err)
	}

	// For now, we'll rely on the ID token claims
	// To get additional userinfo, we'd need the access token which would require
	// modifications to the flow

	return &userInfo, nil
}

// MapOIDCRolesToOVIM maps OIDC roles/groups to OVIM roles
func (p *OIDCProvider) MapOIDCRolesToOVIM(userInfo *UserInfo) string {
	// Check for system admin role
	for _, role := range userInfo.Roles {
		if strings.Contains(strings.ToLower(role), "admin") {
			return "system_admin"
		}
	}

	// Check groups for admin roles
	for _, group := range userInfo.Groups {
		groupLower := strings.ToLower(group)
		if strings.Contains(groupLower, "admin") {
			if strings.Contains(groupLower, "system") {
				return "system_admin"
			}
			return "org_admin"
		}
	}

	// Default to regular user
	return "user"
}

// ValidateOIDCConfig validates OIDC configuration
func ValidateOIDCConfig(config *OIDCConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.IssuerURL == "" {
		return fmt.Errorf("OIDC issuer URL is required when OIDC is enabled")
	}

	if _, err := url.Parse(config.IssuerURL); err != nil {
		return fmt.Errorf("invalid OIDC issuer URL: %w", err)
	}

	if config.ClientID == "" {
		return fmt.Errorf("OIDC client ID is required when OIDC is enabled")
	}

	if config.ClientSecret == "" {
		return fmt.Errorf("OIDC client secret is required when OIDC is enabled")
	}

	if config.RedirectURL == "" {
		return fmt.Errorf("OIDC redirect URL is required when OIDC is enabled")
	}

	if _, err := url.Parse(config.RedirectURL); err != nil {
		return fmt.Errorf("invalid OIDC redirect URL: %w", err)
	}

	return nil
}
