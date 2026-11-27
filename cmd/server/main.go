// Command server runs the Telegram bot and the Mini App HTTP server in one process.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"budgetbot/internal/api"
	"budgetbot/internal/config"
	"budgetbot/internal/store"
	"budgetbot/internal/stt"
	"budgetbot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	var transcriber telegram.Transcriber
	whisper := stt.Whisper{WhisperBin: cfg.WhisperBin, FFmpegBin: cfg.FFmpegBin, Model: cfg.WhisperModel, Language: "uz"}
	if err := whisper.Available(); err != nil {
		slog.Warn("voice messages disabled", "reason", err)
	} else {
		transcriber = whisper
		slog.Info("voice messages enabled", "model", cfg.WhisperModel)
	}

	bot, err := telegram.New(cfg.BotToken, st, cfg.WebAppURL, transcriber)
	if err != nil {
		return err
	}

	var static fs.FS
	if info, err := os.Stat("webapp/dist"); err == nil && info.IsDir() {
		static = os.DirFS("webapp/dist")
	} else {
		slog.Warn("webapp/dist not found; run `bun run build` in webapp/ to serve the Mini App")
	}
	if cfg.DevAuthUserID > 0 {
		slog.Warn("DEV_AUTH_USER_ID is set: API accepts requests without Telegram auth. Never use in production.")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.New(st, cfg.BotToken, cfg.DevAuthUserID).Handler(static),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http", "err", err)
			stop()
		}
	}()

	slog.Info("bot started", "webapp", cfg.WebAppURL)
	bot.Run(ctx) // blocks until ctx is canceled

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
