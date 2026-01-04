package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/grpc_server"
	"go.uber.org/zap"
)

// runDualServer запускает dual сервер (HTTP + gRPC)
func runDualServer(
	application *app.Application,
	grpcServer *grpc_server.VideoStreamServer,
	grpcPort string,
	logger *zap.Logger,
	cfg *config.Config,
) error {
	// Каналы для graceful shutdown
	httpErrChan := make(chan error, 1)
	grpcErrChan := make(chan error, 1)

	// Graceful shutdown контекст
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	// Запуск HTTP сервера
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		logger.Info("🚀 Запуск HTTP сервера",
			zap.String("address", fmt.Sprintf("http://%s", addr)))

		if err := application.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP сервер завершился с ошибкой", zap.Error(err))
			httpErrChan <- err
		}
	}()

	// Запуск gRPC сервера
	go func() {
		logger.Info("🚀 Запуск gRPC сервера",
			zap.String("address", fmt.Sprintf(":%s", grpcPort)))

		if err := grpcServer.Run(grpcPort); err != nil {
			logger.Error("gRPC сервер завершился с ошибкой", zap.Error(err))
			grpcErrChan <- err
		}
	}()

	logger.Info("✅ Сервис запущен в dual режиме (HTTP + gRPC)")
	logger.Info("📡 Доступные интерфейсы:")
	logger.Info(fmt.Sprintf("   HTTP REST API:  http://%s:%d", cfg.Host, cfg.Port))
	logger.Info("   gRPC endpoint:  localhost:" + grpcPort)
	logger.Info(fmt.Sprintf("   Health check:   http://%s:%d/health", cfg.Host, cfg.Port))
	logger.Info("   gRPC reflection: включена")
	logger.Info("")
	logger.Info("📋 Примеры использования:")
	logger.Info("   1. HTTP (Python/REST): POST /api/v1/video/frame")
	logger.Info("   2. gRPC (Go/бинарный): StreamVideo()")
	logger.Info("   3. Тест: curl http://localhost:8080/api/v1/test/endpoints")

	// Ожидание сигнала завершения
	select {
	case <-ctx.Done():
		logger.Info("📴 Получен сигнал завершения...")
	case err := <-httpErrChan:
		logger.Error("Ошибка HTTP сервера", zap.Error(err))
	case err := <-grpcErrChan:
		logger.Error("Ошибка gRPC сервера", zap.Error(err))
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Остановка серверов...")
	if err := application.Stop(); err != nil {
		logger.Error("Ошибка при остановке HTTP сервера", zap.Error(err))
	}

	// Не используем shutdownCtx, но оставляем для cancel()
	_ = shutdownCtx

	logger.Info("✅ Сервис остановлен корректно")
	return nil
}
