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

// UserServiceClient интерфейс для клиента user-service
type UserServiceClient interface {
	Close() error
	GetUserByClientId(ctx context.Context, clientID string) (*userservice.User, error)
	GetStreamingConfig(ctx context.Context, userID string) (*userservice.User_StreamingConfig, error)
	HealthCheck(ctx context.Context) error
	GetUserInfoWithRetry(ctx context.Context, clientID string) (*userservice.User, error)
	Login(ctx context.Context, username, password string) (*userservice.LoginResponse, error)
	ValidateToken(ctx context.Context, token string) (*userservice.ValidateTokenResponse, error)
}

// DefaultUserServiceClient реализация по умолчанию
type DefaultUserServiceClient struct {
	conn   *grpc.ClientConn
	client userservice.UserServiceClient
	logger *zap.Logger
	config *config.Config
}

// NewUserServiceClient создает нового клиента
func NewUserServiceClient(cfg *config.Config, logger *zap.Logger) (UserServiceClient, error) {
	// Пытаемся создать реальное подключение
	realClient, err := newRealUserServiceClient(cfg, logger)
	if err != nil {
		logger.Warn("Failed to create real user-service client, falling back to stub",
			zap.Error(err),
			zap.String("host", cfg.UserService.Host),
			zap.Int("port", cfg.UserService.Port))

		// Возвращаем заглушку если реальный сервис недоступен
		return newStubUserServiceClient(cfg, logger), nil
	}

	return realClient, nil
}

// newRealUserServiceClient создает реальное подключение
func newRealUserServiceClient(cfg *config.Config, logger *zap.Logger) (*DefaultUserServiceClient, error) {
	address := fmt.Sprintf("%s:%d", cfg.UserService.Host, cfg.UserService.Port)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.UserService.DialTimeout)
	defer cancel()

	logger.Info("Connecting to user-service",
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

	return &DefaultUserServiceClient{
		conn:   conn,
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// newStubUserServiceClient создает заглушку для тестирования
func newStubUserServiceClient(cfg *config.Config, logger *zap.Logger) *StubUserServiceClient {
	logger.Info("Using stub user-service client for testing")
	return &StubUserServiceClient{
		logger: logger,
		config: cfg,
	}
}

// Close закрывает соединение
func (c *DefaultUserServiceClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing user-service connection")
		return c.conn.Close()
	}
	return nil
}

// GetUserByClientId получает пользователя по client_id
func (c *DefaultUserServiceClient) GetUserByClientId(ctx context.Context, clientID string) (*userservice.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	// В user-service нет метода GetUserByClientId, используем GetUser
	// Предполагаем, что clientID это user_id
	return c.client.GetUser(ctx, &userservice.GetUserRequest{
		UserId: clientID,
	})
}

// GetStreamingConfig получает конфигурацию стриминга для пользователя
func (c *DefaultUserServiceClient) GetStreamingConfig(ctx context.Context, userID string) (*userservice.User_StreamingConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	// Используем метод GetStreamingConfig из user-service
	return c.client.GetStreamingConfig(ctx, &userservice.GetStreamingConfigRequest{
		UserId:   userID,
		ClientId: "api-gateway", // Идентификатор API Gateway как клиента
	})
}

// HealthCheck проверяет доступность сервиса
func (c *DefaultUserServiceClient) HealthCheck(ctx context.Context) error {
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
func (c *DefaultUserServiceClient) GetUserInfoWithRetry(ctx context.Context, clientID string) (*userservice.User, error) {
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
func (c *DefaultUserServiceClient) Login(ctx context.Context, username, password string) (*userservice.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	return c.client.Login(ctx, &userservice.LoginRequest{
		Username:   username,
		Password:   password,
		ClientInfo: "api-gateway",
	})
}

// ValidateToken проверяет токен
func (c *DefaultUserServiceClient) ValidateToken(ctx context.Context, token string) (*userservice.ValidateTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.UserService.RequestTimeout)
	defer cancel()

	return c.client.ValidateToken(ctx, &userservice.ValidateTokenRequest{
		Token: token,
	})
}

// StubUserServiceClient - заглушка для тестирования
type StubUserServiceClient struct {
	logger *zap.Logger
	config *config.Config
}

func (c *StubUserServiceClient) Close() error {
	c.logger.Debug("Stub user-service client closed")
	return nil
}

func (c *StubUserServiceClient) GetUserByClientId(ctx context.Context, clientID string) (*userservice.User, error) {
	c.logger.Debug("Stub: Getting user by client ID", zap.String("client_id", clientID))

	// Возвращаем заглушечные данные
	return &userservice.User{
		Id:       clientID,
		Username: "user_" + clientID,
		Email:    "user_" + clientID + "@example.com",
		Status:   "active",
		StreamingConfig: &userservice.User_StreamingConfig{
			ServerUrl:      "video-service-1.example.com",
			ServerPort:     8082,
			ApiKey:         "video_api_key_" + clientID,
			StreamEndpoint: "/api/v1/video/stream",
			MaxBitrate:     5000,
			MaxResolution:  1080,
			AllowedCodecs:  []string{"h264", "h265"},
		},
	}, nil
}

func (c *StubUserServiceClient) GetStreamingConfig(ctx context.Context, userID string) (*userservice.User_StreamingConfig, error) {
	c.logger.Debug("Stub: Getting streaming config", zap.String("user_id", userID))

	return &userservice.User_StreamingConfig{
		ServerUrl:      "video-service-1.example.com",
		ServerPort:     8082,
		ApiKey:         "video_api_key_" + userID,
		StreamEndpoint: "/api/v1/video/stream",
		MaxBitrate:     5000,
		MaxResolution:  1080,
		AllowedCodecs:  []string{"h264", "h265"},
	}, nil
}

func (c *StubUserServiceClient) HealthCheck(ctx context.Context) error {
	c.logger.Debug("Stub: Health check")
	return nil
}

func (c *StubUserServiceClient) GetUserInfoWithRetry(ctx context.Context, clientID string) (*userservice.User, error) {
	return c.GetUserByClientId(ctx, clientID)
}

func (c *StubUserServiceClient) Login(ctx context.Context, username, password string) (*userservice.LoginResponse, error) {
	c.logger.Debug("Stub: Login", zap.String("username", username))

	user, _ := c.GetUserByClientId(ctx, username)
	return &userservice.LoginResponse{
		Token: "stub_token_" + username,
		User:  user,
	}, nil
}

func (c *StubUserServiceClient) ValidateToken(ctx context.Context, token string) (*userservice.ValidateTokenResponse, error) {
	c.logger.Debug("Stub: Validate token")

	return &userservice.ValidateTokenResponse{
		Valid: true,
		User: &userservice.User{
			Id:       "stub_user",
			Username: "stub_user",
			Status:   "active",
		},
	}, nil
}
