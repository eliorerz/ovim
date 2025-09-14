package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	t.Run("ValidCertificateGeneration", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// Verify files were created
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)

		// Verify certificate file content
		certPEM, err := ioutil.ReadFile(certFile)
		require.NoError(t, err)
		assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
		assert.Contains(t, string(certPEM), "END CERTIFICATE")

		// Verify key file content
		keyPEM, err := ioutil.ReadFile(keyFile)
		require.NoError(t, err)
		assert.Contains(t, string(keyPEM), "BEGIN PRIVATE KEY")
		assert.Contains(t, string(keyPEM), "END PRIVATE KEY")

		// Parse and verify certificate
		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block)
		assert.Equal(t, "CERTIFICATE", block.Type)

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		// Verify certificate properties
		assert.Equal(t, "OVIM Development", cert.Subject.Organization[0])
		assert.Equal(t, "US", cert.Subject.Country[0])
		assert.Contains(t, cert.DNSNames, "localhost")
		assert.Contains(t, cert.DNSNames, "ovim.local")
		assert.True(t, cert.NotAfter.After(time.Now()))
		assert.True(t, cert.NotBefore.Before(time.Now().Add(time.Minute)))

		// Verify key usage
		assert.True(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
		assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
		assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	})

	t.Run("DirectoryCreation", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create nested directory path
		nestedDir := filepath.Join(tempDir, "nested", "path", "to", "certs")
		certFile := filepath.Join(nestedDir, "cert.pem")
		keyFile := filepath.Join(nestedDir, "key.pem")

		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// Verify directory was created
		assert.DirExists(t, nestedDir)
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("InvalidCertificatePath", func(t *testing.T) {
		// Try to write to a location that should fail (read-only root)
		err := GenerateSelfSignedCert("/invalid/path/cert.pem", "/invalid/path/key.pem")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create certificate directory")
	})

	t.Run("InvalidKeyPath", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")

		// Create a directory where the key file should be (this will cause an error)
		keyFile := filepath.Join(tempDir, "key.pem")
		err = os.Mkdir(keyFile, 0755)
		require.NoError(t, err)

		err = GenerateSelfSignedCert(certFile, keyFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create key file")
	})
}

func TestLoadTLSConfig(t *testing.T) {
	t.Run("ValidCertificates", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Generate test certificates
		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// Load TLS config
		tlsConfig, err := LoadTLSConfig(certFile, keyFile)
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig)

		// Verify TLS configuration
		assert.Len(t, tlsConfig.Certificates, 1)
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
		assert.NotEmpty(t, tlsConfig.CipherSuites)

		// Verify cipher suites
		expectedCiphers := []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		}
		assert.Equal(t, expectedCiphers, tlsConfig.CipherSuites)
	})

	t.Run("NonexistentCertificate", func(t *testing.T) {
		_, err := LoadTLSConfig("nonexistent-cert.pem", "nonexistent-key.pem")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load TLS certificate")
	})

	t.Run("MismatchedCertificateAndKey", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Generate first certificate pair
		cert1File := filepath.Join(tempDir, "cert1.pem")
		key1File := filepath.Join(tempDir, "key1.pem")
		err = GenerateSelfSignedCert(cert1File, key1File)
		require.NoError(t, err)

		// Generate second certificate pair
		cert2File := filepath.Join(tempDir, "cert2.pem")
		key2File := filepath.Join(tempDir, "key2.pem")
		err = GenerateSelfSignedCert(cert2File, key2File)
		require.NoError(t, err)

		// Try to load cert1 with key2 (should fail)
		_, err = LoadTLSConfig(cert1File, key2File)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load TLS certificate")
	})

	t.Run("InvalidCertificateFile", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create invalid certificate file
		certFile := filepath.Join(tempDir, "invalid-cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err = ioutil.WriteFile(certFile, []byte("invalid certificate content"), 0644)
		require.NoError(t, err)

		// Generate valid key
		err = GenerateSelfSignedCert(filepath.Join(tempDir, "temp-cert.pem"), keyFile)
		require.NoError(t, err)

		_, err = LoadTLSConfig(certFile, keyFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load TLS certificate")
	})
}

func TestEnsureCertificates(t *testing.T) {
	t.Run("CertificatesExist", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Generate certificates first
		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// EnsureCertificates should succeed without generating new ones
		err = EnsureCertificates(certFile, keyFile, false)
		assert.NoError(t, err)

		// Files should still exist
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("CertificatesDoNotExistAutoGenerateTrue", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Certificates don't exist, auto-generate enabled
		err = EnsureCertificates(certFile, keyFile, true)
		assert.NoError(t, err)

		// Files should now exist
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("CertificatesDoNotExistAutoGenerateFalse", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Certificates don't exist, auto-generate disabled
		err = EnsureCertificates(certFile, keyFile, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TLS certificate files not found and auto-generation is disabled")

		// Files should not exist
		assert.NoFileExists(t, certFile)
		assert.NoFileExists(t, keyFile)
	})

	t.Run("OnlyCertificateExists", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Create only certificate file
		err = ioutil.WriteFile(certFile, []byte("fake cert"), 0644)
		require.NoError(t, err)

		// Should generate new certificates
		err = EnsureCertificates(certFile, keyFile, true)
		assert.NoError(t, err)

		// Both files should exist
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("OnlyKeyExists", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Create only key file
		err = ioutil.WriteFile(keyFile, []byte("fake key"), 0644)
		require.NoError(t, err)

		// Should generate new certificates
		err = EnsureCertificates(certFile, keyFile, true)
		assert.NoError(t, err)

		// Both files should exist
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("GenerationFailure", func(t *testing.T) {
		// Try to write to invalid location
		err := EnsureCertificates("/invalid/path/cert.pem", "/invalid/path/key.pem", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate TLS certificate")
	})
}

func TestTLSIntegration(t *testing.T) {
	t.Run("GenerateAndLoadCertificates", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// Ensure certificates (should generate new ones)
		err = EnsureCertificates(certFile, keyFile, true)
		require.NoError(t, err)

		// Load TLS config
		tlsConfig, err := LoadTLSConfig(certFile, keyFile)
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig)

		// Verify we can get the certificate from the TLS config
		assert.Len(t, tlsConfig.Certificates, 1)
		cert := tlsConfig.Certificates[0]
		assert.NotNil(t, cert.Certificate)
		assert.NotNil(t, cert.PrivateKey)

		// Parse the certificate to verify its properties
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		require.NoError(t, err)
		assert.Contains(t, x509Cert.DNSNames, "localhost")
		assert.Contains(t, x509Cert.DNSNames, "ovim.local")
	})

	t.Run("MultipleEnsureCallsIdempotent", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		// First call should generate certificates
		err = EnsureCertificates(certFile, keyFile, true)
		require.NoError(t, err)

		// Get file info for comparison
		certInfo1, err := os.Stat(certFile)
		require.NoError(t, err)
		keyInfo1, err := os.Stat(keyFile)
		require.NoError(t, err)

		// Second call should not regenerate
		err = EnsureCertificates(certFile, keyFile, true)
		require.NoError(t, err)

		// Files should be unchanged
		certInfo2, err := os.Stat(certFile)
		require.NoError(t, err)
		keyInfo2, err := os.Stat(keyFile)
		require.NoError(t, err)

		assert.Equal(t, certInfo1.ModTime(), certInfo2.ModTime())
		assert.Equal(t, keyInfo1.ModTime(), keyInfo2.ModTime())
	})
}

func TestCertificateValidity(t *testing.T) {
	t.Run("CertificateValidityPeriod", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// Read and parse certificate
		certPEM, err := ioutil.ReadFile(certFile)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block)

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		// Verify validity period (1 year)
		now := time.Now()
		assert.True(t, cert.NotBefore.Before(now.Add(time.Minute)))
		assert.True(t, cert.NotAfter.After(now.Add(360*24*time.Hour)))  // At least 360 days
		assert.True(t, cert.NotAfter.Before(now.Add(370*24*time.Hour))) // At most 370 days
	})

	t.Run("CertificateIPAddresses", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		// Load TLS config to get the certificate
		tlsConfig, err := LoadTLSConfig(certFile, keyFile)
		require.NoError(t, err)

		cert, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
		require.NoError(t, err)

		// Verify IP addresses
		assert.Len(t, cert.IPAddresses, 2)
		foundIPv4 := false
		foundIPv6 := false
		for _, ip := range cert.IPAddresses {
			if ip.String() == "127.0.0.1" {
				foundIPv4 = true
			}
			if ip.String() == "::1" {
				foundIPv6 = true
			}
		}
		assert.True(t, foundIPv4, "Should contain IPv4 loopback")
		assert.True(t, foundIPv6, "Should contain IPv6 loopback")
	})
}

func TestTLSConfigSecurity(t *testing.T) {
	t.Run("SecureTLSConfiguration", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "tls-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err = GenerateSelfSignedCert(certFile, keyFile)
		require.NoError(t, err)

		tlsConfig, err := LoadTLSConfig(certFile, keyFile)
		require.NoError(t, err)

		// Verify minimum TLS version
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)

		// Verify secure cipher suites are configured
		assert.NotEmpty(t, tlsConfig.CipherSuites)

		// Verify specific secure cipher suites
		secureCSuites := map[uint16]bool{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384: true,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305:  true,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256: true,
		}

		for _, suite := range tlsConfig.CipherSuites {
			assert.True(t, secureCSuites[suite], "Cipher suite %d should be in secure list", suite)
		}
	})
}
