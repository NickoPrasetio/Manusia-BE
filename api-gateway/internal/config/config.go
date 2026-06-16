package config

import "os"

type Config struct {
	Port              string
	AuthServiceURL    string
	UserServiceURL    string
	BookingServiceURL string
	ReviewServiceURL  string
	ChatServiceURL    string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		AuthServiceURL:    getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8082"),
		BookingServiceURL: getEnv("BOOKING_SERVICE_URL", "http://localhost:8083"),
		ReviewServiceURL:  getEnv("REVIEW_SERVICE_URL", "http://localhost:8084"),
		ChatServiceURL:    getEnv("CHAT_SERVICE_URL", "http://localhost:8085"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
