package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	GeminiKey        string `env:"GEMINI_API_KEY"`
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN"`
	DatabaseURL      string `env:"DATABASE_URL"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", cfg); err != nil {
			return nil, fmt.Errorf("failed to parse .env file: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(cfg); err != nil {
			return nil, fmt.Errorf("failed to parse system environment: %w", err)
		}
	}

	if cfg.GeminiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is empty or missing")
	}
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is empty or missing")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is empty or missing")
	}

	return cfg, nil
}
