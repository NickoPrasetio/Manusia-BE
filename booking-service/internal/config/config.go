package config

import "os"

type Config struct {
	Port      string
	DBDSN     string
	JWTSecret string
}

func Load() *Config {
	return &Config{
		Port:      getEnv("PORT", "8083"),
		DBDSN:     getEnv("DB_DSN", "postgres://manusia:manusia_secret@localhost:5432/manusia_bookings?sslmode=disable"),
		JWTSecret: getEnv("JWT_SECRET", "manusia-dev-secret"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
