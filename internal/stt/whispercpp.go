package stt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"noteboard/internal/config"
)

// whisperCLI запускает локальный бинарник whisper.cpp.
type whisperCLI struct{ cfg config.Transcribe }

func newWhisperCLI(cfg config.Transcribe) *whisperCLI { return &whisperCLI{cfg: cfg} }

func (e *whisperCLI) Name() string { return "whisper.cpp" }

func (e *whisperCLI) Transcribe(ctx context.Context, path string) (string, error) {
	args := []string{"-m", e.cfg.WhisperModel, "-f", path, "-nt", "-np"}
	if e.cfg.Language != "" {
		args = append(args, "-l", e.cfg.Language)
	}

	cmd := exec.CommandContext(ctx, e.cfg.WhisperBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper: %w: %s", err, tail(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// tail оставляет от простыни stderr последние строки: в лог и в поле ошибки
// заметки попадает то, что реально помогает.
func tail(s string) string {
	s = strings.TrimSpace(s)
	const limit = 400
	if len(s) > limit {
		return "..." + s[len(s)-limit:]
	}
	return s
}
