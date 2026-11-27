package telegram

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"budgetbot/internal/parser"
)

const (
	maxVoiceSeconds = 60
	maxVoiceBytes   = 5 << 20
)

// onVoice transcribes a voice message and records the expenses in it, like a text message.
func (b *Bot) onVoice(ctx context.Context, m *models.Message) {
	if b.stt == nil {
		if isPrivate(m) {
			b.reply(ctx, m, "🎤 Ovozli xabarlar hozircha o'chirilgan. Xarajatni matn bilan yozing.", nil)
		}
		return
	}
	v := m.Voice
	if v.Duration > maxVoiceSeconds {
		if isPrivate(m) {
			b.reply(ctx, m, fmt.Sprintf("🎤 Ovozli xabar %d soniyadan qisqa bo'lsin.", maxVoiceSeconds), nil)
		}
		return
	}
	if err := b.store.Touch(ctx, m.Chat.ID, chatTitle(m), !isPrivate(m), m.From.ID, m.From.FirstName); err != nil {
		slog.Error("touch", "err", err)
		return
	}

	select {
	case b.voiceSem <- struct{}{}:
		defer func() { <-b.voiceSem }()
	case <-ctx.Done():
		return
	}

	start := time.Now()
	path, err := b.downloadFile(ctx, v.FileID)
	if err != nil {
		slog.Error("download voice", "err", err)
		b.reply(ctx, m, "⚠️ Ovozli xabarni yuklab bo'lmadi.", nil)
		return
	}
	defer os.Remove(path)

	text, err := b.stt.Transcribe(ctx, path)
	if err != nil {
		slog.Error("transcribe", "err", err)
		b.reply(ctx, m, "⚠️ Ovozni matnga aylantirib bo'lmadi.", nil)
		return
	}
	slog.Info("voice transcribed", "chat", m.Chat.ID, "seconds", v.Duration,
		"took", time.Since(start).Round(100*time.Millisecond))

	heard := fmt.Sprintf("🎤 <i>Eshitdim: %s</i>", html.EscapeString(text))
	// In groups, family voice chat that isn't about money is ignored silently.
	items := parser.Parse(text, !isPrivate(m))
	if len(items) == 0 {
		if isPrivate(m) {
			b.reply(ctx, m, heard+"\n\nSummani topa olmadim 🤔 Masalan: <i>\"nonga besh ming\"</i> deb ayting.", nil)
		}
		return
	}
	b.recordItems(ctx, m, items, heard)
}

// downloadFile saves a Telegram file to a temp path and returns it.
func (b *Bot) downloadFile(ctx context.Context, fileID string) (string, error) {
	f, err := b.api.GetFile(ctx, &tg.GetFileParams{FileID: fileID})
	if err != nil {
		return "", err
	}
	if f.FileSize > maxVoiceBytes {
		return "", fmt.Errorf("file too large: %d bytes", f.FileSize)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.api.FileDownloadLink(f), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	out, err := os.CreateTemp("", "voice-*.ogg")
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(resp.Body, maxVoiceBytes+1)); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}
