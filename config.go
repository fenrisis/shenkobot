package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	ChatID        int64
	ReminderHours int
	Question      string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	LogLevel  string
	LogFormat string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}

	chatIDRaw := os.Getenv("CHAT_ID")
	if chatIDRaw == "" {
		return nil, fmt.Errorf("CHAT_ID is required")
	}
	chatID, err := strconv.ParseInt(chatIDRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("CHAT_ID must be int64: %w", err)
	}

	hours := 72
	if v := os.Getenv("REMINDER_HOURS"); v != "" {
		h, err := strconv.Atoi(v)
		if err != nil || h <= 0 {
			return nil, fmt.Errorf("REMINDER_HOURS must be a positive integer")
		}
		hours = h
	}

	question := os.Getenv("QUESTION")
	if question == "" {
		question = "Когда вернешься?"
	}

	return &Config{
		BotToken:      token,
		ChatID:        chatID,
		ReminderHours: hours,
		Question:      question,

		DBHost:     envOr("DB_HOST", "localhost"),
		DBPort:     envOr("DB_PORT", "5432"),
		DBUser:     envOr("DB_USER", "postgres"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     envOr("DB_NAME", "shenkobot"),
		DBSSLMode:  envOr("DB_SSLMODE", "disable"),

		LogLevel:  envOr("LOG_LEVEL", "info"),
		LogFormat: envOr("LOG_FORMAT", "json"),
	}, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

func (c *Config) ReminderInterval() time.Duration {
	return time.Duration(c.ReminderHours) * time.Hour
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
