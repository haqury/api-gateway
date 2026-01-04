package main

import (
	"fmt"
	"os"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/grpc_server"
	"go.uber.org/zap"
)

// runServerCommand запускает сервер
func runServerCommand(debug bool, configPath string, grpcPort string) error {
	// Настройка логгера
	var logger *zap.Logger
	var err error

	if debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Загрузка конфигурации
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Warn("Failed to load config", zap.Error(err))
		cfg = config.GetDefaultConfig()
	}

	// Создание приложения
	application := app.NewApplicationWithConfig(cfg, logger)

	// Создание gRPC сервера
	videoService := app.GetVideoStreamService(application)
	grpcServer := grpc_server.NewVideoStreamServer(videoService, logger)

	// Запуск в dual режиме
	return runDualServer(application, grpcServer, grpcPort, logger, cfg)
}

// runMigrationsCommand запускает миграции
func runMigrationsCommand() error {
	fmt.Println("Migrations not implemented yet")
	return nil
}

// runWorkerCommand запускает воркеры
func runWorkerCommand() error {
	fmt.Println("Workers not implemented yet")
	return nil
}

// runVersionCommand выводит версию
func runVersionCommand() error {
	fmt.Println("API Gateway - Version 1.0.0")
	fmt.Println("Build: dual-api")
	fmt.Println("Git Commit: 92c5e2d")
	return nil
}

// runHealthCheck проверяет здоровье сервиса
func runHealthCheckCommand() error {
	fmt.Println("Health check not implemented yet")
	return nil
}

// runGenerateDocs генерирует документацию
func runGenerateDocsCommand() error {
	fmt.Println("Documentation generation not implemented yet")
	return nil
}

// printHelp выводит справку
func printHelp() {
	fmt.Println(`
API Gateway - Командная строка

Использование:
  api-gateway [команда] [флаги]

Команды:
  server          Запустить сервер (по умолчанию)
  migrate         Выполнить миграции БД
  worker          Запустить фоновых воркеров
  version         Показать версию
  health-check    Проверить здоровье сервиса
  generate-docs   Сгенерировать документацию
  help            Показать эту справку

Флаги для server:
  --debug         Включить debug режим
  --config        Путь к конфигурационному файлу (по умолчанию: ./config/config.yaml)
  --grpc-port     Порт для gRPC сервера (по умолчанию: 9090)

Примеры:
  api-gateway server --debug
  api-gateway server --grpc-port=9091
  api-gateway version
  `)
}

// handleCommand обрабатывает команду
func handleCommand(args []string) error {
	if len(args) == 0 {
		// По умолчанию запускаем сервер
		return runServerCommand(false, "./config/config.yaml", "9090")
	}

	command := args[0]
	switch command {
	case "server":
		// Парсим флаги для server
		debug := false
		configPath := "./config/config.yaml"
		grpcPort := "9090"

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--debug":
				debug = true
			case "--config":
				if i+1 < len(args) {
					configPath = args[i+1]
					i++
				}
			case "--grpc-port":
				if i+1 < len(args) {
					grpcPort = args[i+1]
					i++
				}
			}
		}
		return runServerCommand(debug, configPath, grpcPort)

	case "migrate":
		return runMigrationsCommand()

	case "worker":
		return runWorkerCommand()

	case "version":
		return runVersionCommand()

	case "health-check":
		return runHealthCheckCommand()

	case "generate-docs":
		return runGenerateDocsCommand()

	case "help":
		printHelp()
		return nil

	default:
		fmt.Printf("Неизвестная команда: %s\n", command)
		printHelp()
		os.Exit(1)
		return nil
	}
}
