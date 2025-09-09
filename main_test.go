package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eliorerz/ovim-updated/api"
	"github.com/eliorerz/ovim-updated/config"
	"github.com/gin-gonic/gin"
)

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "OVIM Backend",
			"version": "1.0.0",
		})
	})

	api.SetupRoutes(r, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	expected := `{"service":"OVIM Backend","status":"healthy","version":"1.0.0"}`
	if w.Body.String() != expected {
		t.Errorf("Expected body %s, got %s", expected, w.Body.String())
	}
}
