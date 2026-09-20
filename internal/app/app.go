// Package app собирает приложение из слоёв: конфиг, база, репозитории,
// сценарии, транспорт. Это единственное место, где они видят друг друга —
// ни один слой ниже не знает, кем он подключён.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"noteboard/internal/config"
	"noteboard/internal/controller/rest"
	"noteboard/internal/controller/rpc"
	"noteboard/internal/repo/blob"
	"noteboard/internal/repo/sqlite"
	"noteboard/internal/stt"
	"noteboard/internal/usecase"
	"noteboard/internal/webui"
)

// Run поднимает приложение и возвращается, когда сервер остановлен.
func Run(cfg config.Config) error {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	for _, dir := range cfg.Dirs() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// infrastructure
	db, err := sqlite.Open(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	notesRepo := sqlite.NewNoteRepo(db)
	boardRepo := sqlite.NewBoardRepo(db)
	queueRepo := sqlite.NewQueueRepo(db)
	audio := blob.New(cfg.AudioDir())
	uploads := blob.New(cfg.UploadsDir())

	// После падения незакрытые running возвращаем в очередь.
	if err := queueRepo.ResetRunning(); err != nil {
		return fmt.Errorf("reset queue: %w", err)
	}

	// use cases
	worker := usecase.NewWorker(
		queueRepo,
		stt.NewEngine(cfg.Transcribe),
		stt.NewConverter(cfg.Transcribe.FFmpegBin, cfg.TmpDir()),
		log.With("component", "transcribe"),
		cfg.Transcribe.Timeout,
	)
	notes := usecase.NewNotes(notesRepo, audio, worker)
	board := usecase.NewBoard(boardRepo, notesRepo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go worker.Run(ctx)

	// controller
	static, ok := webui.Handler(cfg.WebDir)
	if !ok {
		log.Warn("frontend не собран, отдаётся только /api")
	}
	api := rest.New(rest.Deps{
		Notes:         notes,
		Uploads:       uploads,
		Static:        static,
		Log:           log,
		Mode:          worker.Mode,
		MaxAudioBytes: cfg.MaxAudioBytes,
		MaxImageBytes: cfg.MaxImageBytes,
	})

	// Два транспорта на одном порту: ConnectRPC под /api/rpc/, всё остальное
	// (файлы, статика, health) — REST. ServeMux выбирает по длине префикса.
	mux := http.NewServeMux()
	rpc.Mount(mux, "/api/rpc", notes, board)
	mux.Handle("/", api)

	// h2c = HTTP/2 без TLS. Нужен только затем, чтобы на этот же порт могли
	// прийти обычные gRPC-клиенты (grpcurl, Go): браузеру хватает HTTP/1.1.
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("старт", "addr", cfg.Addr, "data", cfg.DataDir, "transcribe", cfg.Transcribe.Mode())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	log.Info("остановка", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
