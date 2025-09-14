package util

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringPtr(t *testing.T) {
	t.Run("ValidString", func(t *testing.T) {
		s := "test string"
		ptr := StringPtr(s)
		assert.NotNil(t, ptr)
		assert.Equal(t, s, *ptr)
	})

	t.Run("EmptyString", func(t *testing.T) {
		s := ""
		ptr := StringPtr(s)
		assert.NotNil(t, ptr)
		assert.Equal(t, "", *ptr)
	})

	t.Run("DifferentStrings", func(t *testing.T) {
		s1 := "string1"
		s2 := "string2"
		ptr1 := StringPtr(s1)
		ptr2 := StringPtr(s2)

		assert.NotEqual(t, ptr1, ptr2) // Different pointers
		assert.Equal(t, s1, *ptr1)
		assert.Equal(t, s2, *ptr2)
	})
}

func TestStringValue(t *testing.T) {
	t.Run("ValidPointer", func(t *testing.T) {
		s := "test string"
		ptr := &s
		value := StringValue(ptr)
		assert.Equal(t, s, value)
	})

	t.Run("NilPointer", func(t *testing.T) {
		value := StringValue(nil)
		assert.Equal(t, "", value)
	})

	t.Run("EmptyStringPointer", func(t *testing.T) {
		s := ""
		ptr := &s
		value := StringValue(ptr)
		assert.Equal(t, "", value)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		original := "round trip test"
		ptr := StringPtr(original)
		value := StringValue(ptr)
		assert.Equal(t, original, value)
	})
}

func TestGenerateID(t *testing.T) {
	t.Run("DefaultLength", func(t *testing.T) {
		id, err := GenerateID(DefaultIDLength)
		require.NoError(t, err)
		assert.Len(t, id, DefaultIDLength)
		assert.True(t, isHexString(id))
	})

	t.Run("CustomLength", func(t *testing.T) {
		lengths := []int{4, 6, 10, 16, 32}
		for _, length := range lengths {
			id, err := GenerateID(length)
			require.NoError(t, err)
			assert.Len(t, id, length)
			assert.True(t, isHexString(id))
		}
	})

	t.Run("ZeroLength", func(t *testing.T) {
		id, err := GenerateID(0)
		require.NoError(t, err)
		assert.Len(t, id, DefaultIDLength) // Should use default
		assert.True(t, isHexString(id))
	})

	t.Run("NegativeLength", func(t *testing.T) {
		id, err := GenerateID(-5)
		require.NoError(t, err)
		assert.Len(t, id, DefaultIDLength) // Should use default
		assert.True(t, isHexString(id))
	})

	t.Run("OddLength", func(t *testing.T) {
		id, err := GenerateID(7)
		require.NoError(t, err)
		assert.Len(t, id, 7)
		assert.True(t, isHexString(id))
	})

	t.Run("Uniqueness", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := GenerateID(16)
			require.NoError(t, err)
			assert.False(t, ids[id], "Generated duplicate ID: %s", id)
			ids[id] = true
		}
		assert.Len(t, ids, 100)
	})

	t.Run("LargeLength", func(t *testing.T) {
		id, err := GenerateID(64)
		require.NoError(t, err)
		assert.Len(t, id, 64)
		assert.True(t, isHexString(id))
	})
}

func TestSanitizeKubernetesName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ValidName",
			input:    "valid-name",
			expected: "valid-name",
		},
		{
			name:     "UpperCase",
			input:    "UPPERCASE",
			expected: "uppercase",
		},
		{
			name:     "MixedCase",
			input:    "Mixed-Case-Name",
			expected: "mixed-case-name",
		},
		{
			name:     "InvalidCharacters",
			input:    "name_with_underscores",
			expected: "name-with-underscores",
		},
		{
			name:     "SpecialCharacters",
			input:    "name@with#special$chars",
			expected: "name-with-special-chars",
		},
		{
			name:     "LeadingHyphen",
			input:    "-leading-hyphen",
			expected: "leading-hyphen",
		},
		{
			name:     "TrailingHyphen",
			input:    "trailing-hyphen-",
			expected: "trailing-hyphen",
		},
		{
			name:     "MultipleHyphens",
			input:    "multiple---hyphens",
			expected: "multiple---hyphens",
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: "resource",
		},
		{
			name:     "OnlyInvalidChars",
			input:    "@#$%",
			expected: "resource",
		},
		{
			name:     "OnlyHyphens",
			input:    "---",
			expected: "resource",
		},
		{
			name:     "NumbersAndLetters",
			input:    "test123name456",
			expected: "test123name456",
		},
		{
			name:     "Spaces",
			input:    "name with spaces",
			expected: "name-with-spaces",
		},
		{
			name:     "LongName",
			input:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", KubernetesNameMaxLength),
		},
		{
			name:     "LongNameWithHyphenAtEnd",
			input:    strings.Repeat("a", KubernetesNameMaxLength-1) + "-" + strings.Repeat("b", 10),
			expected: strings.Repeat("a", KubernetesNameMaxLength-1),
		},
		{
			name:     "ComplexInput",
			input:    "MyApp_v1.2.3@domain.com",
			expected: "myapp-v1-2-3-domain-com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeKubernetesName(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.True(t, IsValidKubernetesName(result), "Sanitized name should be valid: %s", result)
		})
	}
}

func TestIsValidKubernetesName(t *testing.T) {
	validNames := []string{
		"valid-name",
		"test123",
		"a",
		"123",
		"name-with-numbers-123",
		"very-long-but-valid-name-that-is-still-under-limit",
		strings.Repeat("a", KubernetesNameMaxLength),
	}

	for _, name := range validNames {
		t.Run("Valid_"+name, func(t *testing.T) {
			assert.True(t, IsValidKubernetesName(name), "Should be valid: %s", name)
		})
	}

	invalidNames := []string{
		"",                      // Empty
		"-leading-hyphen",       // Starts with hyphen
		"trailing-hyphen-",      // Ends with hyphen
		"UPPERCASE",             // Uppercase letters
		"name_with_underscores", // Underscores
		"name with spaces",      // Spaces
		"name@with.special",     // Special characters
		strings.Repeat("a", KubernetesNameMaxLength+1), // Too long
	}

	for _, name := range invalidNames {
		t.Run("Invalid_"+name, func(t *testing.T) {
			assert.False(t, IsValidKubernetesName(name), "Should be invalid: %s", name)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "ShorterThanMax",
			input:     "short",
			maxLength: 10,
			expected:  "short",
		},
		{
			name:      "ExactlyMaxLength",
			input:     "exactly10c",
			maxLength: 10,
			expected:  "exactly10c",
		},
		{
			name:      "LongerThanMax",
			input:     "this is a very long string",
			maxLength: 10,
			expected:  "this is a ",
		},
		{
			name:      "EmptyString",
			input:     "",
			maxLength: 5,
			expected:  "",
		},
		{
			name:      "ZeroMaxLength",
			input:     "test",
			maxLength: 0,
			expected:  "",
		},
		{
			name:      "NegativeMaxLength",
			input:     "test",
			maxLength: -1,
			expected:  "",
		},
		{
			name:      "UnicodeCharacters",
			input:     "cafe",
			maxLength: 3,
			expected:  "caf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLength)
			assert.Equal(t, tt.expected, result)
			if tt.maxLength >= 0 {
				assert.True(t, len(result) <= tt.maxLength, "Result should not exceed max length")
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry", "date"}

	t.Run("ContainsExistingItem", func(t *testing.T) {
		assert.True(t, ContainsString(slice, "banana"))
		assert.True(t, ContainsString(slice, "apple"))
		assert.True(t, ContainsString(slice, "date"))
	})

	t.Run("DoesNotContainMissingItem", func(t *testing.T) {
		assert.False(t, ContainsString(slice, "grape"))
		assert.False(t, ContainsString(slice, ""))
		assert.False(t, ContainsString(slice, "APPLE"))
	})

	t.Run("EmptySlice", func(t *testing.T) {
		emptySlice := []string{}
		assert.False(t, ContainsString(emptySlice, "anything"))
	})

	t.Run("NilSlice", func(t *testing.T) {
		var nilSlice []string
		assert.False(t, ContainsString(nilSlice, "anything"))
	})

	t.Run("SliceWithEmptyString", func(t *testing.T) {
		sliceWithEmpty := []string{"a", "", "b"}
		assert.True(t, ContainsString(sliceWithEmpty, ""))
		assert.True(t, ContainsString(sliceWithEmpty, "a"))
		assert.False(t, ContainsString(sliceWithEmpty, "c"))
	})

	t.Run("SliceWithDuplicates", func(t *testing.T) {
		sliceWithDups := []string{"a", "b", "a", "c"}
		assert.True(t, ContainsString(sliceWithDups, "a"))
		assert.True(t, ContainsString(sliceWithDups, "b"))
		assert.False(t, ContainsString(sliceWithDups, "d"))
	})
}

func TestRemoveString(t *testing.T) {
	t.Run("RemoveExistingItem", func(t *testing.T) {
		original := []string{"apple", "banana", "cherry", "date"}
		result := RemoveString(original, "banana")
		expected := []string{"apple", "cherry", "date"}
		assert.Equal(t, expected, result)
		assert.NotContains(t, result, "banana")
	})

	t.Run("RemoveNonExistingItem", func(t *testing.T) {
		original := []string{"apple", "banana", "cherry"}
		result := RemoveString(original, "grape")
		assert.Equal(t, original, result)
	})

	t.Run("RemoveFromEmptySlice", func(t *testing.T) {
		original := []string{}
		result := RemoveString(original, "anything")
		assert.Empty(t, result)
	})

	t.Run("RemoveAllOccurrences", func(t *testing.T) {
		original := []string{"a", "b", "a", "c", "a"}
		result := RemoveString(original, "a")
		expected := []string{"b", "c"}
		assert.Equal(t, expected, result)
		assert.NotContains(t, result, "a")
	})

	t.Run("RemoveEmptyString", func(t *testing.T) {
		original := []string{"a", "", "b", "", "c"}
		result := RemoveString(original, "")
		expected := []string{"a", "b", "c"}
		assert.Equal(t, expected, result)
		assert.NotContains(t, result, "")
	})

	t.Run("RemoveLastItem", func(t *testing.T) {
		original := []string{"only"}
		result := RemoveString(original, "only")
		assert.Empty(t, result)
	})

	t.Run("OriginalSliceUnmodified", func(t *testing.T) {
		original := []string{"apple", "banana", "cherry"}
		originalCopy := make([]string, len(original))
		copy(originalCopy, original)

		_ = RemoveString(original, "banana")

		// Original should be unchanged
		assert.Equal(t, originalCopy, original)
	})

	t.Run("ResultCapacity", func(t *testing.T) {
		original := []string{"a", "b", "c", "d", "e"}
		result := RemoveString(original, "c")

		// Result should have proper capacity
		assert.Equal(t, len(original), cap(result))
		assert.Len(t, result, 4)
	})
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 8, DefaultIDLength)
	assert.Equal(t, 63, KubernetesNameMaxLength)
}

func TestStringUtilsIntegration(t *testing.T) {
	t.Run("IDGenerationAndValidation", func(t *testing.T) {
		// Generate ID
		id, err := GenerateID(8)
		require.NoError(t, err)

		// Use in slice operations
		slice := []string{"id1", "id2", id}
		assert.True(t, ContainsString(slice, id))

		// Remove it
		newSlice := RemoveString(slice, id)
		assert.False(t, ContainsString(newSlice, id))
		assert.Len(t, newSlice, 2)
	})

	t.Run("SanitizeAndValidate", func(t *testing.T) {
		input := "My Application_v1.0@Company.com"
		sanitized := SanitizeKubernetesName(input)

		// Should be valid after sanitization
		assert.True(t, IsValidKubernetesName(sanitized))

		// Should be within length limit
		assert.True(t, len(sanitized) <= KubernetesNameMaxLength)

		// Should contain only valid characters
		assert.Regexp(t, `^[a-z0-9\-]+$`, sanitized)
	})

	t.Run("TruncateAndPointers", func(t *testing.T) {
		longString := strings.Repeat("a", 100)
		truncated := TruncateString(longString, 50)

		// Convert to pointer and back
		ptr := StringPtr(truncated)
		value := StringValue(ptr)

		assert.Equal(t, truncated, value)
		assert.Len(t, value, 50)
	})
}

// Helper function to check if a string is valid hexadecimal
func isHexString(s string) bool {
	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return hexRegex.MatchString(s)
}
