package app

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"api-gateway/internal/config"
	"api-gateway/internal/controller"
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
}

// NewApplicationWithConfig создает новое приложение с конфигурацией
func NewApplicationWithConfig(cfg *config.Config, logger *zap.Logger) *Application {
	// Создаем сервисы
	clientInfoService := controller.NewClientInfoService(logger)
	videoStreamService := controller.NewVideoStreamService(logger)

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
	}
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

// Start запускает приложение
func (app *Application) Start() error {
	app.logger.Info("Starting application",
		zap.String("address", app.server.Addr))

	return app.server.ListenAndServe()
}

// Stop останавливает приложение
func (app *Application) Stop() error {
	app.logger.Info("Stopping application")
	return app.server.Close()
}

// GetRouter возвращает роутер
func (app *Application) GetRouter() http.Handler {
	return app.router
}
