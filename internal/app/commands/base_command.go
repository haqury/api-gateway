package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CommandContext содержит общий контекст для всех команд
type CommandContext struct {
	Logger *zap.Logger
	Config *Config
}

// Config конфигурация приложения
type Config struct {
	Port         int
	Host         string
	Debug        bool
	DBConnection string
	RedisURL     string
	LogLevel     string
}

// NewCommandContext создает новый контекст команды
func NewCommandContext(c *cli.Context) (*CommandContext, error) {
	// Настраиваем логгер
	logLevel := c.String("log-level")
	logger := createLogger(logLevel)

	// Загружаем конфигурацию
	config := loadConfig(c)

	return &CommandContext{
		Logger: logger,
		Config: config,
	}, nil
}

// createLogger создает логгер
func createLogger(level string) *zap.Logger {
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zap.DebugLevel
	case "warn":
		logLevel = zap.WarnLevel
	case "error":
		logLevel = zap.ErrorLevel
	default:
		logLevel = zap.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel)

	logger, _ := config.Build()
	return logger
}

// loadConfig загружает конфигурацию
func loadConfig(c *cli.Context) *Config {
	return &Config{
		Port:         c.Int("port"),
		Host:         c.String("host"),
		Debug:        c.Bool("debug"),
		DBConnection: c.String("db"),
		RedisURL:     c.String("redis"),
		LogLevel:     c.String("log-level"),
	}
}
