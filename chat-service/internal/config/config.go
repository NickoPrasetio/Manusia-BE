package config

import "os"

type Config struct {
	Port           string
	DBDSN          string
	JWTSecret      string
	MinioEndpoint  string
	MinioAccess    string
	MinioSecret    string
	MinioBucket    string
	MinioPublicURL string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8085"),
		DBDSN:          getEnv("DB_DSN", "postgres://manusia:manusia_secret@localhost:5432/manusia_chats?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "manusia-dev-secret"),
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:29000"),
		MinioAccess:    getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecret:    getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		MinioBucket:    getEnv("MINIO_BUCKET", "manusia-chat-photos"),
		MinioPublicURL: getEnv("MINIO_PUBLIC_URL", "http://localhost:29000"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
