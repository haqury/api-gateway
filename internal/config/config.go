package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config представляет конфигурацию приложения
type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	// gRPC
	GRPCPort string `yaml:"grpc_port"`

	// Database
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Name     string `yaml:"name"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"database"`

	// Redis
	Redis struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	// JWT
	JWT struct {
		Secret     string `yaml:"secret"`
		Expiration int    `yaml:"expiration"`
	} `yaml:"jwt"`

	// Logging
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`

	// Video settings
	Video struct {
		MaxFrameSize int    `yaml:"max_frame_size"`
		MaxFPS       int    `yaml:"max_fps"`
		Codec        string `yaml:"codec"`
	} `yaml:"video"`

	// User Service gRPC client configuration
	UserService struct {
		Host           string        `yaml:"host"`
		Port           int           `yaml:"port"`
		DialTimeout    time.Duration `yaml:"dial_timeout"`
		RequestTimeout time.Duration `yaml:"request_timeout"`
		MaxRetries     int           `yaml:"max_retries"`
		RetryDelay     time.Duration `yaml:"retry_delay"`
	} `yaml:"user_service"`
}

// LoadConfig загружает конфигурацию из файла
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetDefaultConfig возвращает конфигурацию по умолчанию
func GetDefaultConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     8080,
		GRPCPort: "9090",
		Database: struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			Name     string `yaml:"name"`
			SSLMode  string `yaml:"ssl_mode"`
		}{
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "postgres",
			Name:     "api_gateway",
			SSLMode:  "disable",
		},
		Redis: struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
		}{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		JWT: struct {
			Secret     string `yaml:"secret"`
			Expiration int    `yaml:"expiration"`
		}{
			Secret:     "your-secret-key-change-in-production",
			Expiration: 24,
		},
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  "info",
			Format: "json",
		},
		Video: struct {
			MaxFrameSize int    `yaml:"max_frame_size"`
			MaxFPS       int    `yaml:"max_fps"`
			Codec        string `yaml:"codec"`
		}{
			MaxFrameSize: 10 * 1024 * 1024, // 10MB
			MaxFPS:       30,
			Codec:        "h264",
		},
		UserService: struct {
			Host           string        `yaml:"host"`
			Port           int           `yaml:"port"`
			DialTimeout    time.Duration `yaml:"dial_timeout"`
			RequestTimeout time.Duration `yaml:"request_timeout"`
			MaxRetries     int           `yaml:"max_retries"`
			RetryDelay     time.Duration `yaml:"retry_delay"`
		}{
			Host:           "localhost",
			Port:           9091, // Порт user-service gRPC
			DialTimeout:    10 * time.Second,
			RequestTimeout: 5 * time.Second,
			MaxRetries:     3,
			RetryDelay:     1 * time.Second,
		},
	}
}
