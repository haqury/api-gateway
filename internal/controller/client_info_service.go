package controller

import (
	"context"
	"time"

	pb "api-gateway/pkg/gen"

	helpy "github.com/haqury/helpy"
	"go.uber.org/zap"
)

// ClientInfoServiceImpl - реализация сервиса
type ClientInfoServiceImpl struct {
	logger *zap.Logger
	repo   *ClientRepository
}

// NewClientInfoService создает новый сервис
func NewClientInfoService(logger *zap.Logger) *ClientInfoServiceImpl {
	return &ClientInfoServiceImpl{
		logger: logger,
		repo:   NewClientRepository(),
	}
}

// ClientConnected - клиент подключился
func (s *ClientInfoServiceImpl) ClientConnected(ctx context.Context, req *pb.ConnectionEvent) (*helpy.ApiResponse, error) {
	s.logger.Info("Client connected",
		zap.String("client_id", req.ClientId),
		zap.String("ip", req.IpAddress))

	// Сохраняем клиента
	s.repo.SaveClient(req.ClientInfo)

	return &helpy.ApiResponse{
		Status:    "ok",
		Message:   "Client connected successfully",
		Timestamp: time.Now().Unix(),
	}, nil
}

// ClientDisconnected - клиент отключился
func (s *ClientInfoServiceImpl) ClientDisconnected(ctx context.Context, req *pb.ConnectionEvent) (*helpy.ApiResponse, error) {
	s.logger.Info("Client disconnected",
		zap.String("client_id", req.ClientId))

	// Удаляем или обновляем статус
	s.repo.RemoveClient(req.ClientId)

	return &helpy.ApiResponse{
		Status:    "ok",
		Message:   "Client disconnected",
		Timestamp: time.Now().Unix(),
	}, nil
}

// UpdateClientInfo - обновить информацию о клиенте
func (s *ClientInfoServiceImpl) UpdateClientInfo(ctx context.Context, req *pb.UpdateClientRequest) (*helpy.ApiResponse, error) {
	s.logger.Info("Updating client info",
		zap.String("client_id", req.ClientId))

	if req.ClientInfo != nil {
		s.repo.SaveClient(req.ClientInfo)
	}

	return &helpy.ApiResponse{
		Status:    "ok",
		Message:   "Client info updated",
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetClientInfo - получить информацию о клиенте
func (s *ClientInfoServiceImpl) GetClientInfo(ctx context.Context, req *pb.GetClientInfoRequest) (*pb.ClientInfo, error) {
	s.logger.Debug("Getting client info",
		zap.String("client_id", req.ClientId))

	client := s.repo.GetClient(req.ClientId)
	if client == nil {
		return nil, nil // Возвращаем nil, если клиент не найден
	}

	return client, nil
}

// ListActiveClients - список активных клиентов
func (s *ClientInfoServiceImpl) ListActiveClients(ctx context.Context, req *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	s.logger.Debug("Listing active clients")

	allClients := s.repo.GetAllClients()

	// Конвертируем int32 в int для операций сравнения
	page := int(req.Page)
	limit := int(req.Limit)
	totalClients := len(allClients)

	// Простая пагинация
	start := (page - 1) * limit
	end := start + limit

	if start >= totalClients {
		return &pb.ListClientsResponse{
			Clients: []*pb.ClientInfo{},
			Total:   int32(totalClients),
		}, nil
	}

	if end > totalClients {
		end = totalClients
	}

	return &pb.ListClientsResponse{
		Clients: allClients[start:end],
		Total:   int32(totalClients),
	}, nil
}
