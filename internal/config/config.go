package config

import "os"

type Config struct {
	WSAllowedOrigins string
	Port             string
	DatabaseURL      string
	RedisURL         string
	JWTSecret        string
	LLMBaseURL       string
	LLMAPIKey        string
	LLMModel         string
	SandboxImage     string
	WorkspacePath    string
	SearxngURL       string
	EmbedAPIKey      string
	EmbedBaseURL     string
}

func Load() *Config {
	return &Config{
		WSAllowedOrigins: getEnv("WS_ALLOWED_ORIGINS", ""),
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://grokagent:secret@localhost:5432/grokagent"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		LLMBaseURL:       getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:        getEnv("LLM_API_KEY", ""),
		LLMModel:         getEnv("LLM_MODEL", "gpt-4o"),
		SandboxImage:     getEnv("SANDBOX_IMAGE", "grok-agent-sandbox:latest"),
		WorkspacePath:    getEnv("WORKSPACE_PATH", ""),
		SearxngURL:       getEnv("SEARXNG_URL", "http://localhost:8888"),
		EmbedAPIKey:      getEnv("EMBED_API_KEY", ""),
		EmbedBaseURL:     getEnv("EMBED_BASE_URL", "https://api.openai.com/v1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
