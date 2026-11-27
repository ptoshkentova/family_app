// Package stt turns voice messages into text with a local whisper.cpp install.
package stt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prompt nudges Whisper toward budget vocabulary and digits ("5 ming" rather than "besh ming").
const prompt = "Xarajatlar: non 5 ming, taksi 20 ming, bozor 150 ming so'm, dori 45 ming, oylik 6 mln."

type Whisper struct {
	WhisperBin string // whisper-cli
	FFmpegBin  string // ffmpeg
	Model      string // path to a ggml model file
	Language   string // "uz"
}

// Available reports whether the binaries and model exist, so the bot can disable voice gracefully.
func (w Whisper) Available() error {
	for _, bin := range []string{w.WhisperBin, w.FFmpegBin} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found", bin)
		}
	}
	if _, err := os.Stat(w.Model); err != nil {
		return fmt.Errorf("model %s not found", w.Model)
	}
	return nil
}

// Transcribe converts any audio file ffmpeg can read (Telegram voice is OGG/Opus) to text.
func (w Whisper) Transcribe(ctx context.Context, audioPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	dir, err := os.MkdirTemp("", "voice-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "audio.wav")

	// whisper.cpp expects 16 kHz mono 16-bit PCM WAV.
	if out, err := exec.CommandContext(ctx, w.FFmpegBin, "-nostdin", "-loglevel", "error", "-i", audioPath,
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav).CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg: %v: %s", err, bytes.TrimSpace(out))
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, w.WhisperBin, "-m", w.Model, "-l", w.Language, "-f", wav,
		"-nt", "-np", "--prompt", prompt)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper: %v: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return strings.Join(strings.Fields(stdout.String()), " "), nil
}
