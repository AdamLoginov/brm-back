package config

import (
	"os"

	"github.com/joho/godotenv"
)

var JWTKey []byte

func LoadConfig() {
	_ = godotenv.Load()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "fallback_key_for_dev"
	}

	JWTKey = []byte(secret)
}
