package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL      string
	KubernetesConfig string
	JWTSecret        string
	Port             string
	Environment      string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      getEnv("OVIM_DATABASE_URL", "postgres://ovim:ovim@localhost/ovim?sslmode=disable"),
		KubernetesConfig: getEnv("OVIM_KUBECONFIG", ""),
		JWTSecret:        getEnv("OVIM_JWT_SECRET", "ovim-default-secret-change-in-production"),
		Port:             getEnv("OVIM_PORT", "8080"),
		Environment:      getEnv("OVIM_ENVIRONMENT", "development"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
