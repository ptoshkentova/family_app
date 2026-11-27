package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	DatabaseURL   string
	HTTPAddr      string
	WebAppURL     string
	DevAuthUserID int64 // 0 = disabled

	// Voice messages (local whisper.cpp). Disabled automatically if the model is missing.
	WhisperModel string
	WhisperBin   string
	FFmpegBin    string
}

func Load() (Config, error) {
	_ = godotenv.Load() // .env is optional; real env vars win
	cfg := Config{
		BotToken:    strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		DatabaseURL: getenv("DATABASE_URL", "postgres://localhost:5433/budget?sslmode=disable"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		WebAppURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("WEBAPP_URL")), "/"),

		WhisperModel: getenv("WHISPER_MODEL", "models/ggml-univoice-stt-uz-q5_0.bin"),
		WhisperBin:   getenv("WHISPER_BIN", "whisper-cli"),
		FFmpegBin:    getenv("FFMPEG_BIN", "ffmpeg"),
	}
	if cfg.BotToken == "" {
		return cfg, errors.New("BOT_TOKEN is not set (copy .env.example to .env)")
	}
	if cfg.WebAppURL != "" && !strings.HasPrefix(cfg.WebAppURL, "https://") {
		return cfg, errors.New("WEBAPP_URL must start with https:// (Telegram requirement)")
	}
	if v := strings.TrimSpace(os.Getenv("DEV_AUTH_USER_ID")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return cfg, errors.New("DEV_AUTH_USER_ID must be a number")
		}
		cfg.DevAuthUserID = id
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
