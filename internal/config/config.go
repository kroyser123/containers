package config

import (
	"os"
)

type Config struct {
	DatabaseURL     string
	OwnerBotToken   string
	PublicBaseURL   string
	LLMBaseURL      string
	LLMAPIKey       string
	LLMModel        string
	LLMFallbackModel string
	AppSecret       string
	HTTPAddr        string
}

func Load() Config {
	return Config{
		DatabaseURL:      env("DATABASE_URL", "postgres://aiinbox:aiinbox@localhost:5432/aiinbox?sslmode=disable"),
		OwnerBotToken:    env("OWNER_BOT_TOKEN", ""),
		PublicBaseURL:    env("PUBLIC_BASE_URL", "http://localhost:8080"),
		LLMBaseURL:       env("LLM_BASE_URL", "https://openrouter.ai/api/v1"),
		LLMAPIKey:        env("LLM_API_KEY", ""),
		LLMModel:         env("LLM_MODEL", "openai/gpt-4o-mini"),
		LLMFallbackModel: env("LLM_FALLBACK_MODEL", ""),
		AppSecret:        env("APP_SECRET", "dev-secret-change-me"),
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
