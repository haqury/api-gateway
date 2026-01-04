package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Если переданы флаги командной строки
	if len(os.Args) > 1 && os.Args[1][0] != '-' {
		// Обрабатываем как команду
		if err := handleCommand(os.Args[1:]); err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Иначе парсим флаги для backward compatibility
	mode := flag.String("mode", "server", "Режим работы: server, migrate, worker, version")
	debug := flag.Bool("debug", false, "Включить debug режим")
	configPath := flag.String("config", "./config/config.yaml", "Путь к конфигурационному файлу")
	grpcPort := flag.String("grpc-port", "9090", "Порт для gRPC сервера")

	flag.Parse()

	switch *mode {
	case "server":
		if err := runServerCommand(*debug, *configPath, *grpcPort); err != nil {
			fmt.Printf("Ошибка запуска сервера: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		runMigrationsCommand()
	case "worker":
		runWorkerCommand()
	case "version":
		fmt.Printf("API Gateway\n")
		fmt.Printf("Version:    %s\n", Version)
		fmt.Printf("Commit:     %s\n", Commit)
		fmt.Printf("Build Date: %s\n", BuildDate)
	default:
		fmt.Printf("Неизвестный режим: %s\n", *mode)
		os.Exit(1)
	}
}
