package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	GeminiKey        string `env:"GEMINI_API_KEY" env-required:"true"`
	TelegramBotToken string `env:"TElEGRAM_BOT_TOKEN" env-required:"true"`
	DatabaseURL      string `env:"DATABASE_URL" env-required:"true"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	if err := cleanenv.ReadConfig(".env", cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment configuration: %w", err)
	}

	return cfg, nil
}
