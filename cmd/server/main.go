package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/httpapi"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	config, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		logger.Error("配置无效", "error", err)
		os.Exit(2)
	}
	if config.SelfCheck {
		if err := runSelfCheck(context.Background()); err != nil {
			logger.Error("自检失败", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := runServer(config, logger); err != nil {
		logger.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func runServer(config config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := storage.Open(ctx, storage.Options{Path: config.DataPath})
	if err != nil {
		return err
	}
	defer store.Close()
	service := workflow.New(store, workflow.Options{})
	api := httpapi.New(service, logger)
	server := &http.Server{
		Addr: config.Address, Handler: api.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
	}
	result := make(chan error, 1)
	go func() {
		logger.Info("服务启动", "addr", config.Address, "data", config.DataPath)
		result <- server.ListenAndServe()
	}()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("收到停止信号，开始优雅关闭")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
