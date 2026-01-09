package grpc_client

import (
	"context"
	"fmt"
	"time"

	"api-gateway/internal/config"
	userservice "github.com/haqury/user-service/pkg/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealUserServiceClient - реальный gRPC клиент для user-service
type RealUserServiceClient struct {
	conn   *grpc.ClientConn
	client userservice.UserServiceClient
	logger *zap.Logger
	config *config.Config
}

// NewRealUserServiceClient создает нового реального клиента
func NewRealUserServiceClient(cfg *config.Config, logger *zap.Logger) (*RealUserServiceClient, error) {
	address := fmt.Sprintf("%s:%d", cfg.UserService.Host, cfg.UserService.Port)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.UserService.DialTimeout)
	defer cancel()

	logger.Info("Connecting to real user-service",
		zap.String("address", address),
		zap.Duration("timeout", cfg.UserService.DialTimeout))

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user-service at %s: %w", address, err)
	}

	client := userservice.NewUserServiceClient(conn)

	logger.Info("Successfully connected to user-service",
		zap.String("address", address))

	return &RealUserServiceClient{
		conn:   conn,
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// Close закрывает соединение
func (c *RealUserServiceClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing user-service connection")
		return c.conn.Close()
	}
	return nil
}

// GetUserByClientId получает пользователя по client_id
func (c *RealUserServiceClient) GetUserByClientId(ctx context.Context, clientID string) (*userservice.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	// В user-service нет метода GetUserByClientId, используем GetUser
	// Предполагаем, что clientID это user_id
	return c.client.GetUser(ctx, &userservice.GetUserRequest{
		UserId: clientID,
	})
}

// GetStreamingConfig получает конфигурацию стриминга для пользователя
func (c *RealUserServiceClient) GetStreamingConfig(ctx context.Context, userID string) (*userservice.User_StreamingConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	// Используем метод GetStreamingConfig из user-service
	return c.client.GetStreamingConfig(ctx, &userservice.GetStreamingConfigRequest{
		UserId:   userID,
		ClientId: "api-gateway", // Идентификатор API Gateway как клиента
	})
}

// HealthCheck проверяет доступность сервиса
func (c *RealUserServiceClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Пытаемся вызвать простой метод для проверки
	_, err := c.client.GetUser(ctx, &userservice.GetUserRequest{
		UserId: "health_check",
	})

	// Ожидаем ошибку "not found", что означает что сервис доступен
	if err != nil && err.Error() != "rpc error: code = NotFound desc = user not found" {
		return fmt.Errorf("user-service health check failed: %w", err)
	}

	return nil
}

// GetUserInfoWithRetry получает информацию о пользователе с повторными попытками
func (c *RealUserServiceClient) GetUserInfoWithRetry(ctx context.Context, clientID string) (*userservice.User, error) {
	var lastErr error

	for i := 0; i < c.config.UserService.MaxRetries; i++ {
		user, err := c.GetUserByClientId(ctx, clientID)
		if err == nil {
			return user, nil
		}

		lastErr = err
		c.logger.Warn("Failed to get user info, retrying",
			zap.String("client_id", clientID),
			zap.Int("attempt", i+1),
			zap.Error(err))

		if i < c.config.UserService.MaxRetries-1 {
			time.Sleep(c.config.UserService.RetryDelay)
		}
	}

	return nil, fmt.Errorf("failed to get user info after %d attempts: %w", c.config.UserService.MaxRetries, lastErr)
}

// Login выполняет аутентификацию
func (c *RealUserServiceClient) Login(ctx context.Context, username, password string) (*userservice.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	return c.client.Login(ctx, &userservice.LoginRequest{
		Username:   username,
		Password:   password,
		ClientInfo: "api-gateway",
	})
}

// ValidateToken проверяет токен
func (c *RealUserServiceClient) ValidateToken(ctx context.Context, token string) (*userservice.ValidateTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	return c.client.ValidateToken(ctx, &userservice.ValidateTokenRequest{
		Token: token,
	})
}
