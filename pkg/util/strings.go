package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const (
	// DefaultIDLength is the default length for generated IDs
	DefaultIDLength = 8

	// KubernetesNameMaxLength is the maximum length for Kubernetes resource names
	KubernetesNameMaxLength = 63
)

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// StringValue returns the value of a string pointer, or empty string if nil
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GenerateID generates a random hexadecimal ID of the specified length
func GenerateID(length int) (string, error) {
	if length <= 0 {
		length = DefaultIDLength
	}

	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	id := hex.EncodeToString(bytes)
	if len(id) > length {
		id = id[:length]
	}

	return id, nil
}

// SanitizeKubernetesName converts a string to a valid Kubernetes resource name
func SanitizeKubernetesName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	name = reg.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it doesn't start or end with a hyphen
	if strings.HasPrefix(name, "-") {
		name = strings.TrimPrefix(name, "-")
	}
	if strings.HasSuffix(name, "-") {
		name = strings.TrimSuffix(name, "-")
	}

	// Truncate if too long
	if len(name) > KubernetesNameMaxLength {
		name = name[:KubernetesNameMaxLength]
		// Ensure it doesn't end with a hyphen after truncation
		name = strings.TrimSuffix(name, "-")
	}

	// Ensure it's not empty
	if name == "" {
		name = "resource"
	}

	return name
}

// IsValidKubernetesName checks if a string is a valid Kubernetes resource name
func IsValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > KubernetesNameMaxLength {
		return false
	}

	// Must start and end with alphanumeric character
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(name) {
		return false
	}
	if !regexp.MustCompile(`[a-z0-9]$`).MatchString(name) {
		return false
	}

	// Can only contain lowercase alphanumeric characters and hyphens
	if !regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString(name) {
		return false
	}

	return true
}

// TruncateString truncates a string to the specified maximum length
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength]
}

// ContainsString checks if a slice contains a specific string
func ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveString removes a string from a slice
func RemoveString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
