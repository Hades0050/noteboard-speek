// Package config собирает все настройки в одном месте. Правило простое:
// os.Getenv встречается только здесь, остальной код получает готовую структуру.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr            string        // адрес HTTP-сервера
	DataDir         string        // корень для базы и файлов
	WebDir          string        // отдавать фронт из папки вместо вшитого
	ShutdownTimeout time.Duration // сколько ждать активные запросы при остановке
	MaxAudioBytes   int64
	MaxImageBytes   int64
	Transcribe      Transcribe
}

// Transcribe — настройки распознавания речи. Пустой APIURL и пустой WhisperBin
// означают, что расшифровка выключена; это допустимый режим, а не ошибка.
type Transcribe struct {
	APIURL   string // OpenAI-совместимый /v1/audio/transcriptions
	APIKey   string
	Model    string
	Language string

	Prompt   string // имена и термины: модель перестаёт их коверкать
	Hotwords string // то же, но точнее; понимает faster-whisper, не OpenAI
	VAD      string // "true" — резать тишину, меньше галлюцинаций на паузах

	WhisperBin   string // whisper.cpp CLI
	WhisperModel string // путь к ggml-модели
	FFmpegBin    string
	Timeout      time.Duration // потолок на одну расшифровку
}

// Mode отвечает на вопрос «чем вообще расшифровываем» — его показывает /api/health.
func (t Transcribe) Mode() string {
	switch {
	case t.APIURL != "":
		return "api"
	case t.WhisperBin != "" && t.WhisperModel != "":
		return "whisper.cpp"
	default:
		return "off"
	}
}

func (c Config) DBPath() string     { return filepath.Join(c.DataDir, "noteboard.db") }
func (c Config) AudioDir() string   { return filepath.Join(c.DataDir, "audio") }
func (c Config) UploadsDir() string { return filepath.Join(c.DataDir, "uploads") }
func (c Config) TmpDir() string     { return filepath.Join(c.DataDir, "tmp") }

// Dirs — всё, что нужно создать при старте.
func (c Config) Dirs() []string {
	return []string{c.DataDir, c.AudioDir(), c.UploadsDir(), c.TmpDir()}
}

// Load читает окружение и проверяет его. Ошибка конфигурации должна падать
// на старте, а не через час в середине запроса.
func Load() (Config, error) {
	c := Config{
		Addr:            env("ADDR", ":8080"),
		DataDir:         env("DATA_DIR", "./data"),
		WebDir:          os.Getenv("WEB_DIR"),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		MaxAudioBytes:   envBytes("MAX_AUDIO_BYTES", 64<<20),
		MaxImageBytes:   envBytes("MAX_IMAGE_BYTES", 16<<20),
		Transcribe: Transcribe{
			APIURL:       os.Getenv("TRANSCRIBE_API_URL"),
			APIKey:       os.Getenv("TRANSCRIBE_API_KEY"),
			Model:        env("TRANSCRIBE_MODEL", "whisper-1"),
			Language:     env("TRANSCRIBE_LANG", "ru"),
			Prompt:       os.Getenv("TRANSCRIBE_PROMPT"),
			Hotwords:     os.Getenv("TRANSCRIBE_HOTWORDS"),
			VAD:          os.Getenv("TRANSCRIBE_VAD"),
			WhisperBin:   os.Getenv("WHISPER_BIN"),
			WhisperModel: os.Getenv("WHISPER_MODEL"),
			FFmpegBin:    env("FFMPEG_BIN", "ffmpeg"),
			Timeout:      envDuration("TRANSCRIBE_TIMEOUT", 30*time.Minute),
		},
	}
	return c, c.validate()
}

func (c Config) validate() error {
	if c.Addr == "" {
		return fmt.Errorf("ADDR пустой")
	}
	if c.DataDir == "" {
		return fmt.Errorf("DATA_DIR пустой")
	}
	if c.MaxAudioBytes <= 0 || c.MaxImageBytes <= 0 {
		return fmt.Errorf("лимиты загрузки должны быть больше нуля")
	}
	t := c.Transcribe
	if t.WhisperBin != "" && t.WhisperModel == "" {
		return fmt.Errorf("WHISPER_BIN задан без WHISPER_MODEL")
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envBytes(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
