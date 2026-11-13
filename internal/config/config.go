package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var JWTKey []byte

func LoadConfig() {
	_ = godotenv.Load()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if os.Getenv("APP_ENV") == "production" {
			log.Fatal("JWT_SECRET is required in production")
		}
		secret = "fallback_key_for_dev"
	}

	JWTKey = []byte(secret)
}
