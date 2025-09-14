package version

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	t.Run("BasicVersionInfo", func(t *testing.T) {
		info := Get()

		// Test static values
		assert.Equal(t, "1", info.Major)
		assert.Equal(t, "0", info.Minor)

		// Test runtime values
		assert.Equal(t, runtime.Version(), info.GoVersion)
		assert.Equal(t, runtime.Compiler, info.Compiler)
		expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
		assert.Equal(t, expectedPlatform, info.Platform)

		// Test default build-time values (when not set by build process)
		assert.Contains(t, info.GitVersion, "v0.0.0-master")
		assert.Contains(t, info.GitCommit, "$Format:")
		assert.Equal(t, "", info.GitTreeState)
		assert.Equal(t, "1970-01-01T00:00:00Z", info.BuildDate)
	})

	t.Run("RuntimeInformation", func(t *testing.T) {
		info := Get()

		// Verify Go version format
		assert.True(t, strings.HasPrefix(info.GoVersion, "go"))

		// Verify compiler is set
		assert.NotEmpty(t, info.Compiler)
		assert.True(t, info.Compiler == "gc" || info.Compiler == "gccgo")

		// Verify platform format
		assert.Contains(t, info.Platform, "/")
		parts := strings.Split(info.Platform, "/")
		assert.Len(t, parts, 2)
		assert.NotEmpty(t, parts[0]) // OS
		assert.NotEmpty(t, parts[1]) // Architecture
	})

	t.Run("ConsistentResults", func(t *testing.T) {
		info1 := Get()
		info2 := Get()

		// Multiple calls should return identical information
		assert.Equal(t, info1, info2)
	})
}

func TestInfoString(t *testing.T) {
	t.Run("StringRepresentation", func(t *testing.T) {
		info := Get()
		stringRep := info.String()

		// String representation should be the git version
		assert.Equal(t, info.GitVersion, stringRep)
		assert.Contains(t, stringRep, "v0.0.0-master")
	})

	t.Run("CustomInfo", func(t *testing.T) {
		customInfo := Info{
			GitVersion: "v1.2.3",
		}

		assert.Equal(t, "v1.2.3", customInfo.String())
	})
}

func TestInfoJSONSerialization(t *testing.T) {
	t.Run("JSONMarshaling", func(t *testing.T) {
		info := Get()

		jsonData, err := json.Marshal(info)
		require.NoError(t, err)

		// Verify JSON contains expected fields
		jsonStr := string(jsonData)
		assert.Contains(t, jsonStr, "major")
		assert.Contains(t, jsonStr, "minor")
		assert.Contains(t, jsonStr, "gitVersion")
		assert.Contains(t, jsonStr, "gitCommit")
		assert.Contains(t, jsonStr, "gitTreeState")
		assert.Contains(t, jsonStr, "buildDate")
		assert.Contains(t, jsonStr, "goVersion")
		assert.Contains(t, jsonStr, "compiler")
		assert.Contains(t, jsonStr, "platform")

		// Verify values are properly serialized
		assert.Contains(t, jsonStr, "\"major\":\"1\"")
		assert.Contains(t, jsonStr, "\"minor\":\"0\"")
	})

	t.Run("JSONUnmarshaling", func(t *testing.T) {
		originalInfo := Get()

		// Marshal to JSON
		jsonData, err := json.Marshal(originalInfo)
		require.NoError(t, err)

		// Unmarshal back to Info struct
		var unmarshaledInfo Info
		err = json.Unmarshal(jsonData, &unmarshaledInfo)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, originalInfo.Major, unmarshaledInfo.Major)
		assert.Equal(t, originalInfo.Minor, unmarshaledInfo.Minor)
		assert.Equal(t, originalInfo.GitVersion, unmarshaledInfo.GitVersion)
		assert.Equal(t, originalInfo.GitCommit, unmarshaledInfo.GitCommit)
		assert.Equal(t, originalInfo.GitTreeState, unmarshaledInfo.GitTreeState)
		assert.Equal(t, originalInfo.BuildDate, unmarshaledInfo.BuildDate)
		assert.Equal(t, originalInfo.GoVersion, unmarshaledInfo.GoVersion)
		assert.Equal(t, originalInfo.Compiler, unmarshaledInfo.Compiler)
		assert.Equal(t, originalInfo.Platform, unmarshaledInfo.Platform)
	})
}

func TestInfoStruct(t *testing.T) {
	t.Run("InfoStructCreation", func(t *testing.T) {
		info := Info{
			Major:        "2",
			Minor:        "1",
			GitVersion:   "v2.1.0",
			GitCommit:    "abc123def456",
			GitTreeState: "clean",
			BuildDate:    "2023-01-01T12:00:00Z",
			GoVersion:    "go1.20.0",
			Compiler:     "gc",
			Platform:     "linux/amd64",
		}

		assert.Equal(t, "2", info.Major)
		assert.Equal(t, "1", info.Minor)
		assert.Equal(t, "v2.1.0", info.GitVersion)
		assert.Equal(t, "abc123def456", info.GitCommit)
		assert.Equal(t, "clean", info.GitTreeState)
		assert.Equal(t, "2023-01-01T12:00:00Z", info.BuildDate)
		assert.Equal(t, "go1.20.0", info.GoVersion)
		assert.Equal(t, "gc", info.Compiler)
		assert.Equal(t, "linux/amd64", info.Platform)
	})

	t.Run("EmptyInfoStruct", func(t *testing.T) {
		var info Info

		assert.Empty(t, info.Major)
		assert.Empty(t, info.Minor)
		assert.Empty(t, info.GitVersion)
		assert.Empty(t, info.GitCommit)
		assert.Empty(t, info.GitTreeState)
		assert.Empty(t, info.BuildDate)
		assert.Empty(t, info.GoVersion)
		assert.Empty(t, info.Compiler)
		assert.Empty(t, info.Platform)

		// String should return empty GitVersion
		assert.Empty(t, info.String())
	})
}

func TestBuildTimeVariables(t *testing.T) {
	t.Run("DefaultBuildVariables", func(t *testing.T) {
		// These tests verify the default values when build-time variables are not set
		info := Get()

		// gitVersion should contain the default template
		assert.Contains(t, info.GitVersion, "v0.0.0-master")
		assert.Contains(t, info.GitVersion, "$Format:")

		// gitCommit should contain the template format
		assert.Contains(t, info.GitCommit, "$Format:")

		// gitTreeState should be empty by default
		assert.Empty(t, info.GitTreeState)

		// buildDate should be the Unix epoch
		assert.Equal(t, "1970-01-01T00:00:00Z", info.BuildDate)
	})
}

func TestVersionConsistency(t *testing.T) {
	t.Run("MultipleCallsConsistent", func(t *testing.T) {
		// Get version info multiple times
		infos := make([]Info, 5)
		for i := range infos {
			infos[i] = Get()
		}

		// All should be identical
		for i := 1; i < len(infos); i++ {
			assert.Equal(t, infos[0], infos[i])
		}
	})

	t.Run("StringConsistency", func(t *testing.T) {
		info := Get()
		str1 := info.String()
		str2 := info.String()

		assert.Equal(t, str1, str2)
		assert.Equal(t, info.GitVersion, str1)
	})
}

func TestPlatformInformation(t *testing.T) {
	t.Run("PlatformFormat", func(t *testing.T) {
		info := Get()

		// Platform should be in format "OS/ARCH"
		parts := strings.Split(info.Platform, "/")
		assert.Len(t, parts, 2)

		os := parts[0]
		arch := parts[1]

		assert.NotEmpty(t, os)
		assert.NotEmpty(t, arch)

		// Note: We don't assert specific values since tests might run on different platforms
		// But we verify the format is correct
		assert.True(t, len(os) > 0, "OS should not be empty")
		assert.True(t, len(arch) > 0, "Architecture should not be empty")

		// The actual runtime values should match
		assert.Equal(t, runtime.GOOS, os)
		assert.Equal(t, runtime.GOARCH, arch)
	})
}

func TestGoVersionInformation(t *testing.T) {
	t.Run("GoVersionFormat", func(t *testing.T) {
		info := Get()

		// Go version should start with "go"
		assert.True(t, strings.HasPrefix(info.GoVersion, "go"))

		// Should match runtime.Version()
		assert.Equal(t, runtime.Version(), info.GoVersion)

		// Should contain version number
		assert.True(t, len(info.GoVersion) > 2)
	})

	t.Run("CompilerInformation", func(t *testing.T) {
		info := Get()

		// Compiler should be set
		assert.NotEmpty(t, info.Compiler)

		// Should match runtime.Compiler
		assert.Equal(t, runtime.Compiler, info.Compiler)

		// Most common compilers
		assert.True(t, info.Compiler == "gc" || info.Compiler == "gccgo")
	})
}
