package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	GinMode          string
	Database         DatabaseConfig
	Logto            LogtoConfig
	JWT              JWTConfig
	Session          SessionConfig
	ExportSigningKey string
	AdminUsers       []string
	TestMode         bool
}

type DatabaseConfig struct {
	URL string
}

type LogtoConfig struct {
	Endpoint         string
	AppID            string
	AppSecret        string
	RedirectURI      string
	PostLogoutURI    string
}

type JWTConfig struct {
	Secret string
}

type SessionConfig struct {
	Secret string
	Secure bool
}

func Load() (*Config, error) {
	godotenv.Load()

	adminUsersStr := os.Getenv("ADMIN_USERS")
	adminUsers := []string{}
	if adminUsersStr != "" {
		adminUsers = strings.Split(adminUsersStr, ",")
		for i := range adminUsers {
			adminUsers[i] = strings.TrimSpace(adminUsers[i])
		}
	}

	return &Config{
		Port:    getEnv("PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "debug"),
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		Logto: LogtoConfig{
			Endpoint:      getEnv("LOGTO_ENDPOINT", ""),
			AppID:         getEnv("LOGTO_APP_ID", ""),
			AppSecret:     getEnv("LOGTO_APP_SECRET", ""),
			RedirectURI:   getEnv("LOGTO_REDIRECT_URI", ""),
			PostLogoutURI: getEnv("LOGTO_POST_LOGOUT_URI", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		Session: SessionConfig{
			Secret: getEnv("SESSION_SECRET", ""),
			Secure: getEnv("SESSION_SECURE", "false") == "true",
		},
		ExportSigningKey: getEnv("EXPORT_SIGNING_KEY", ""),
		AdminUsers:       adminUsers,
		TestMode:         getEnv("TEST_MODE", "false") == "true",
	}, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
