package main

import (
	"log"
	"net/http"
	"os"

	"github.com/eliorerz/ovim-updated/api"
	"github.com/eliorerz/ovim-updated/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "OVIM Backend",
			"version": "1.0.0",
		})
	})

	api.InitStorage()
	api.SetupRoutes(r, cfg)

	port := os.Getenv("OVIM_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting OVIM backend server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
