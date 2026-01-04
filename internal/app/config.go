package app

import (
	"os"
	"strconv"
)

// Config содержит конфигурацию приложения
type Config struct {
	Port         int
	Debug        bool
	DBConnection string
	LogLevel     string
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() *Config {
	port := 8080
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	debug := false
	if envDebug := os.Getenv("DEBUG"); envDebug != "" {
		if d, err := strconv.ParseBool(envDebug); err == nil {
			debug = d
		}
	}

	return &Config{
		Port:         port,
		Debug:        debug,
		DBConnection: os.Getenv("DB_CONNECTION"),
		LogLevel:     os.Getenv("LOG_LEVEL"),
	}
}
