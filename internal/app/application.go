package app

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"api-gateway/internal/config"
	"api-gateway/internal/controller"
	"api-gateway/internal/grpc_client"
	"api-gateway/internal/handler"
)

// Application - основное приложение
type Application struct {
	config             *config.Config
	logger             *zap.Logger
	router             http.Handler
	server             *http.Server
	clientInfoService  *controller.ClientInfoServiceImpl
	videoStreamService *controller.VideoStreamServiceImpl
	clientInfoHandler  *handler.ClientInfoHandler
	videoStreamHandler *handler.VideoStreamHandler
	userClient         grpc_client.UserServiceClient
}

// NewApplicationWithConfig создает новое приложение с конфигурацией
func NewApplicationWithConfig(cfg *config.Config, logger *zap.Logger) (*Application, error) {
	// Создаем gRPC клиент для user-service
	userClient, err := grpc_client.NewUserServiceClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create user service client: %w", err)
	}

	// Создаем сервисы
	clientInfoService := controller.NewClientInfoService(logger)
	videoStreamService := controller.NewVideoStreamService(logger, userClient)

	// Создаем хендлеры
	clientInfoHandler := handler.NewClientInfoHandler(logger, clientInfoService)
	videoStreamHandler := handler.NewVideoStreamHandler(logger, videoStreamService)

	// Создаем роутер
	router := NewRouter(clientInfoHandler, videoStreamHandler, logger)

	// Настраиваем HTTP сервер
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	return &Application{
		config:             cfg,
		logger:             logger,
		router:             router,
		server:             server,
		clientInfoService:  clientInfoService,
		videoStreamService: videoStreamService,
		clientInfoHandler:  clientInfoHandler,
		videoStreamHandler: videoStreamHandler,
		userClient:         userClient,
	}, nil
}

// GetVideoStreamService возвращает видеосервис
func GetVideoStreamService(app *Application) *controller.VideoStreamServiceImpl {
	return app.videoStreamService
}

// GetClientInfoService возвращает клиентский сервис
func GetClientInfoService(app *Application) *controller.ClientInfoServiceImpl {
	return app.clientInfoService
}

// GetConfig возвращает конфигурацию
func GetConfig(app *Application) *config.Config {
	return app.config
}

// GetUserClient возвращает user-service клиент
func GetUserClient(app *Application) grpc_client.UserServiceClient {
	return app.userClient
}

// Start запускает приложение
func (app *Application) Start() error {
	app.logger.Info("Starting application",
		zap.String("address", app.server.Addr),
		zap.String("user_service", fmt.Sprintf("%s:%d", app.config.UserService.Host, app.config.UserService.Port)))

	return app.server.ListenAndServe()
}

// Stop останавливает приложение
func (app *Application) Stop() error {
	app.logger.Info("Stopping application")

	// Закрываем user-service клиент
	if app.userClient != nil {
		if err := app.userClient.Close(); err != nil {
			app.logger.Error("Failed to close user service client", zap.Error(err))
		}
	}

	return app.server.Close()
}

// GetRouter возвращает роутер
func (app *Application) GetRouter() http.Handler {
	return app.router
}
