package stt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Converter приводит запись к формату, который понимают все движки.
type Converter struct {
	bin    string
	tmpDir string
}

func NewConverter(bin, tmpDir string) *Converter { return &Converter{bin: bin, tmpDir: tmpDir} }

// ToWAV конвертирует запись в 16 кГц моно PCM. Браузер отдаёт webm/opus без
// длительности и Cues, и часть STT-серверов такой файл просто не декодирует.
// Вызывающий обязан вызвать cleanup.
func (c *Converter) ToWAV(ctx context.Context, path string) (out string, cleanup func(), err error) {
	out = filepath.Join(c.tmpDir, fmt.Sprintf("conv-%d.wav", time.Now().UnixNano()))
	cmd := exec.CommandContext(ctx, c.bin,
		"-nostdin", "-y", "-i", path, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", out)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(out)
		if errors.Is(err, exec.ErrNotFound) {
			return "", func() {}, errors.New("нужен ffmpeg для конвертации webm в wav")
		}
		return "", func() {}, fmt.Errorf("ffmpeg: %w: %s", err, tail(stderr.String()))
	}
	return out, func() { os.Remove(out) }, nil
}
