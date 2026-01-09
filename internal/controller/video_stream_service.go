package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api-gateway/internal/grpc_client"
	pb "api-gateway/pkg/gen"
	helpy "github.com/haqury/helpy"
	userservice "github.com/haqury/user-service/pkg/gen"
	"go.uber.org/zap"
)

// VideoStreamServiceImpl - сервис для управления видеостримами
type VideoStreamServiceImpl struct {
	repo         *StreamRepository
	logger       *zap.Logger
	userClient   grpc_client.UserServiceClient
	mu           sync.RWMutex
	videoTargets map[string]*VideoTarget // Кэш целей для видео-сервисов
}

// VideoTarget информация о видео-сервисе для пользователя
type VideoTarget struct {
	ServerURL string
	Port      int
	APIKey    string
	Endpoint  string
}

// NewVideoStreamService создает новый сервис
func NewVideoStreamService(
	logger *zap.Logger,
	userClient grpc_client.UserServiceClient,
) *VideoStreamServiceImpl {
	return &VideoStreamServiceImpl{
		repo:         NewStreamRepository(),
		logger:       logger,
		userClient:   userClient,
		videoTargets: make(map[string]*VideoTarget),
	}
}

// getVideoTarget получает или создает цель для видео-сервиса пользователя
func (s *VideoStreamServiceImpl) getVideoTarget(clientID string) (*VideoTarget, error) {
	s.mu.RLock()
	target, exists := s.videoTargets[clientID]
	s.mu.RUnlock()

	if exists {
		return target, nil
	}

	// Получаем конфигурацию стриминга для пользователя
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := s.userClient.GetStreamingConfig(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get streaming config for client %s: %w", clientID, err)
	}

	// Преобразуем конфигурацию в VideoTarget
	target = &VideoTarget{
		ServerURL: config.ServerUrl,
		Port:      int(config.ServerPort),
		APIKey:    config.ApiKey,
		Endpoint:  config.StreamEndpoint,
	}

	s.mu.Lock()
	s.videoTargets[clientID] = target
	s.mu.Unlock()

	return target, nil
}

// getUserInfo получает информацию о пользователе
func (s *VideoStreamServiceImpl) getUserInfo(ctx context.Context, clientID string) (*userservice.User, error) {
	return s.userClient.GetUserByClientId(ctx, clientID)
}

// StartStream - начало стрима с проверкой пользователя
func (s *VideoStreamServiceImpl) StartStream(
	ctx context.Context,
	req *pb.StartStreamRequest,
) (*pb.StartStreamResponse, error) {
	s.logger.Info("Starting stream",
		zap.String("client_id", req.ClientId),
		zap.String("user_id", req.UserId),
		zap.String("camera", req.CameraName))

	// 1. Проверяем пользователя и получаем информацию
	user, err := s.getUserInfo(ctx, req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("user not found or unauthorized: %w", err)
	}

	// 2. Получаем цель для видео-сервиса
	target, err := s.getVideoTarget(req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("failed to get video service target: %w", err)
	}

	// 3. Запускаем стрим
	streamID := fmt.Sprintf("stream_%s_%d", req.ClientId, time.Now().UnixNano())

	// 4. Сохраняем информацию о стриме
	activeStream := &pb.ActiveStream{
		StreamId:    streamID,
		ClientId:    req.ClientId,
		UserName:    user.Username,
		CameraName:  req.CameraName,
		IsRecording: true,
		IsStreaming: true,
		Metadata: map[string]string{
			"video_server":   fmt.Sprintf("%s:%d", target.ServerURL, target.Port),
			"video_endpoint": target.Endpoint,
			"api_key":        target.APIKey,
			"max_bitrate":    fmt.Sprintf("%d", user.StreamingConfig.MaxBitrate),
			"max_resolution": fmt.Sprintf("%d", user.StreamingConfig.MaxResolution),
			"user_id":        user.Id,
			"username":       user.Username,
		},
	}

	s.repo.SaveStream(streamID, activeStream)

	s.logger.Info("Stream started with video service target",
		zap.String("stream_id", streamID),
		zap.String("client_id", req.ClientId),
		zap.String("video_server", fmt.Sprintf("%s:%d", target.ServerURL, target.Port)),
		zap.String("username", user.Username))

	return &pb.StartStreamResponse{
		StreamId: streamID,
		Status:   "started",
		Message:  fmt.Sprintf("Stream %s started for user %s", streamID, user.Username),
		Metadata: map[string]string{
			"video_server": fmt.Sprintf("%s:%d", target.ServerURL, target.Port),
			"user":         user.Username,
			"endpoint":     target.Endpoint,
			"user_id":      user.Id,
		},
	}, nil
}

// SendFrameInternal - отправка кадра с маршрутизацией на видео-сервис
func (s *VideoStreamServiceImpl) SendFrameInternal(
	streamID, clientID, userName string,
	frame *pb.VideoFrame,
) (*helpy.ApiResponse, error) {
	if frame == nil {
		return &helpy.ApiResponse{
			Status:  "error",
			Message: "Frame is nil",
		}, nil
	}

	// 1. Получаем информацию о стриме
	s.mu.RLock()
	stream := s.repo.GetStream(streamID)
	s.mu.RUnlock()

	// 2. Если стрима нет, создаем его с проверкой пользователя
	if stream == nil {
		s.logger.Info("Auto-creating stream with user validation",
			zap.String("stream_id", streamID),
			zap.String("client_id", clientID))

		// Проверяем пользователя
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user, err := s.getUserInfo(ctx, clientID)
		if err != nil {
			return &helpy.ApiResponse{
				Status:  "error",
				Message: fmt.Sprintf("User validation failed: %v", err),
			}, nil
		}

		// Получаем цель для видео-сервиса
		target, err := s.getVideoTarget(clientID)
		if err != nil {
			return &helpy.ApiResponse{
				Status:  "error",
				Message: fmt.Sprintf("Failed to get video service target: %v", err),
			}, nil
		}

		// Создаем стрим
		activeStream := &pb.ActiveStream{
			StreamId:    streamID,
			ClientId:    clientID,
			UserName:    user.Username,
			CameraName:  "auto_created",
			IsRecording: true,
			IsStreaming: true,
			Metadata: map[string]string{
				"video_server":   fmt.Sprintf("%s:%d", target.ServerURL, target.Port),
				"video_endpoint": target.Endpoint,
				"api_key":        target.APIKey,
				"user_id":        user.Id,
				"username":       user.Username,
			},
		}

		s.mu.Lock()
		s.repo.SaveStream(streamID, activeStream)
		s.mu.Unlock()

		stream = activeStream
	}

	// 3. Получаем информацию о видео-сервисе из метаданных стрима
	videoServer, ok := stream.Metadata["video_server"]
	if !ok {
		return &helpy.ApiResponse{
			Status:  "error",
			Message: "Video server not found in stream metadata",
		}, nil
	}

	// 4. Отправляем кадр на видео-сервис (здесь должна быть реальная отправка)
	// TODO: Реализовать отправку на видео-сервис
	s.logger.Debug("Frame would be sent to video service",
		zap.String("stream_id", streamID),
		zap.String("client_id", clientID),
		zap.String("video_server", videoServer),
		zap.Int64("frame_size", int64(len(frame.FrameData))),
		zap.String("username", stream.UserName))

	// 5. Обновляем статистику локально
	stats := s.repo.UpdateStats(streamID, frame)

	return &helpy.ApiResponse{
		Status:    "ok",
		Message:   fmt.Sprintf("Frame routed to %s", videoServer),
		Timestamp: time.Now().Unix(),
		Metadata: map[string]string{
			"stream_id":       streamID,
			"client_id":       clientID,
			"video_server":    videoServer,
			"frames_received": fmt.Sprintf("%d", stats.GetFramesReceived()),
			"bytes_received":  fmt.Sprintf("%d", stats.GetBytesReceived()),
			"source":          "api_gateway",
			"target":          "video_service",
			"routed":          "true",
			"username":        stream.UserName,
			"user_id":         stream.Metadata["user_id"],
		},
	}, nil
}

// SendFrame - отправка кадра (для обратной совместимости)
func (s *VideoStreamServiceImpl) SendFrame(
	ctx context.Context,
	req *pb.SendFrameRequest,
) (*helpy.ApiResponse, error) {
	return s.SendFrameInternal(
		req.StreamId,
		req.ClientId,
		req.UserName,
		req.Frame,
	)
}

// StopStream - остановка стрима
func (s *VideoStreamServiceImpl) StopStream(
	ctx context.Context,
	req *pb.StopStreamRequest,
) (*helpy.ApiResponse, error) {
	s.logger.Info("Stopping stream",
		zap.String("stream_id", req.StreamId),
		zap.String("client_id", req.ClientId))

	// Получаем информацию о стриме для логирования
	stream := s.repo.GetStream(req.StreamId)
	if stream != nil {
		videoServer, ok := stream.Metadata["video_server"]
		if ok {
			s.logger.Info("Stopping stream with video service",
				zap.String("video_server", videoServer),
				zap.String("username", stream.UserName))
		}
	}

	s.repo.RemoveStream(req.StreamId)

	// Удаляем из кэша целей
	s.mu.Lock()
	delete(s.videoTargets, req.ClientId)
	s.mu.Unlock()

	return &helpy.ApiResponse{
		Status:    "ok",
		Message:   fmt.Sprintf("Stream %s stopped", req.StreamId),
		Timestamp: time.Now().Unix(),
		Metadata: map[string]string{
			"stream_id": req.StreamId,
			"client_id": req.ClientId,
			"end_time":  fmt.Sprintf("%d", req.EndTime),
			"file_size": fmt.Sprintf("%d", req.FileSize),
			"filename":  req.Filename,
		},
	}, nil
}

// GetStreamStats - получение статистики стрима
func (s *VideoStreamServiceImpl) GetStreamStats(
	ctx context.Context,
	req *pb.GetStreamStatsRequest,
) (*pb.StreamStats, error) {
	stats := s.repo.GetStats(req.StreamId)
	if stats == nil {
		return nil, fmt.Errorf("stream %s not found", req.StreamId)
	}
	return stats, nil
}

// GetAllActiveStreams - получение всех активных стримов
func (s *VideoStreamServiceImpl) GetAllActiveStreams() []*pb.ActiveStream {
	return s.repo.GetAllActiveStreams()
}

// GetAllStats возвращает всю статистику
func (s *VideoStreamServiceImpl) GetAllStats() []*pb.StreamStats {
	return s.repo.GetAllStats()
}

// GetStreamsByClient возвращает стримы клиента
func (s *VideoStreamServiceImpl) GetStreamsByClient(clientID string) []*pb.ActiveStream {
	allStreams := s.repo.GetAllStreams()
	var clientStreams []*pb.ActiveStream

	for _, stream := range allStreams {
		if stream.ClientId == clientID {
			clientStreams = append(clientStreams, stream)
		}
	}

	return clientStreams
}

// GetActiveStreamsCount возвращает количество активных стримов
func (s *VideoStreamServiceImpl) GetActiveStreamsCount() int {
	return len(s.repo.GetAllActiveStreams())
}

// GetTotalStats - общая статистика
func (s *VideoStreamServiceImpl) GetTotalStats() map[string]interface{} {
	allStats := s.repo.GetAllStats()

	var totalFrames int64
	var totalBytes int64

	for _, stats := range allStats {
		totalFrames += stats.FramesReceived
		totalBytes += stats.BytesReceived
	}

	return map[string]interface{}{
		"active_streams": len(allStats),
		"total_frames":   totalFrames,
		"total_bytes":    totalBytes,
		"average_fps":    calculateAverageFPS(allStats),
		"timestamp":      time.Now().Unix(),
	}
}

func calculateAverageFPS(stats []*pb.StreamStats) float32 {
	if len(stats) == 0 {
		return 0
	}

	var totalFPS float32
	for _, s := range stats {
		totalFPS += s.AverageFps
	}

	return totalFPS / float32(len(stats))
}
