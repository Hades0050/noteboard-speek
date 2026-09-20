package usecase

// Фоновая расшифровка: воркер разбирает очередь заметок и складывает текст
// обратно в хранилище. Движок и конвертер приходят интерфейсами.

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"noteboard/internal/entity"
)

// pollInterval — страховка на случай пропущенного сигнала Notify.
const pollInterval = 15 * time.Second

// Worker разбирает очередь по одной заметке. Параллелить нечего: и локальная
// модель, и внешний API всё равно упираются в одну очередь на сервере.
type Worker struct {
	queue   TranscribeQueue
	engine  Engine
	conv    Converter
	log     *slog.Logger
	timeout time.Duration
	wake    chan struct{}
}

func NewWorker(queue TranscribeQueue, engine Engine, conv Converter, log *slog.Logger, timeout time.Duration) *Worker {
	return &Worker{
		queue:   queue,
		engine:  engine,
		conv:    conv,
		log:     log,
		timeout: timeout,
		// Буфер на один сигнал: пока воркер занят, лишние Notify не копятся.
		wake: make(chan struct{}, 1),
	}
}

// Mode показывает, чем расшифровываем; это же значение отдаёт /api/health.
func (w *Worker) Mode() string {
	if w.engine == nil {
		return "off"
	}
	return w.engine.Name()
}

// Notify будит воркер и никогда не блокирует вызывающего.
func (w *Worker) Notify() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run крутится до отмены контекста. Вызывать в отдельной горутине.
func (w *Worker) Run(ctx context.Context) {
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()

	for {
		w.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-tick.C:
		}
	}
}

// drain обрабатывает очередь до пустоты.
func (w *Worker) drain(ctx context.Context) {
	for {
		note, err := w.queue.Claim()
		if errors.Is(err, entity.ErrNotFound) {
			return
		}
		if err != nil {
			w.log.Error("не смог взять заметку из очереди", "err", err)
			return
		}

		text, failure := w.process(ctx, note)
		if failure != nil {
			w.log.Warn("расшифровка не удалась", "note", note.ID, "err", failure)
		} else {
			w.log.Info("расшифровано", "note", note.ID, "runes", len([]rune(text)))
		}
		if err := w.queue.Finish(note.ID, text, failure); err != nil {
			w.log.Error("не смог сохранить результат", "note", note.ID, "err", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (w *Worker) process(ctx context.Context, note entity.Note) (string, error) {
	if w.engine == nil {
		return "", errors.New("расшифровка не настроена: задай TRANSCRIBE_API_URL или WHISPER_BIN+WHISPER_MODEL")
	}
	if note.AudioPath == "" {
		return "", errors.New("нет аудиофайла")
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	path := note.AudioPath
	wav, cleanup, err := w.conv.ToWAV(ctx, path)
	switch {
	case err == nil:
		defer cleanup()
		path = wav
	case w.engine.Name() == "whisper.cpp":
		// whisper.cpp принимает только WAV, без конвертации дальше идти незачем.
		return "", err
	default:
		// Сервер может справиться и с исходником — пробуем, но пишем в лог.
		w.log.Warn("конвертация не удалась, отправляю исходник", "note", note.ID, "err", err)
	}

	return w.engine.Transcribe(ctx, path)
}
