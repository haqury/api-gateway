package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "api-gateway/internal/gen"
	"go.uber.org/zap"
)

// VideoStreamServiceImpl - сервис для управления видеостримами
type VideoStreamServiceImpl struct {
	repo   *StreamRepository
	logger *zap.Logger
	mu     sync.RWMutex
}

// NewVideoStreamService создает новый сервис
func NewVideoStreamService(logger *zap.Logger) *VideoStreamServiceImpl {
	return &VideoStreamServiceImpl{
		repo:   NewStreamRepository(),
		logger: logger,
	}
}

// StartStream - начало стрима
func (s *VideoStreamServiceImpl) StartStream(
	ctx context.Context,
	req *pb.StartStreamRequest,
) (*pb.StartStreamResponse, error) {
	s.logger.Info("Starting stream",
		zap.String("client_id", req.ClientId),
		zap.String("camera", req.CameraName))

	streamID := fmt.Sprintf("stream_%s_%d", req.ClientId, time.Now().UnixNano())

	activeStream := &pb.ActiveStream{
		StreamId:    streamID,
		ClientId:    req.ClientId,
		UserName:    req.UserId,
		CameraName:  req.CameraName,
		IsRecording: true,
		IsStreaming: true,
	}

	s.repo.SaveStream(streamID, activeStream)

	return &pb.StartStreamResponse{
		StreamId: streamID,
		Status:   "started",
		Message:  fmt.Sprintf("Stream %s started", streamID),
	}, nil
}

// SendFrame - отправка кадра (для обратной совместимости)
func (s *VideoStreamServiceImpl) SendFrame(
	ctx context.Context,
	req *pb.SendFrameRequest,
) (*pb.ApiResponse, error) {
	return s.SendFrameInternal(
		req.StreamId,
		req.ClientId,
		req.UserName,
		req.Frame,
	)
}

// SendFrameInternal - внутренний метод для обработки кадра
func (s *VideoStreamServiceImpl) SendFrameInternal(
	streamID, clientID, userName string,
	frame *pb.VideoFrame,
) (*pb.ApiResponse, error) {
	if frame == nil {
		return &pb.ApiResponse{
			Status:  "error",
			Message: "Frame is nil",
		}, nil
	}

	// Автоматически создаем стрим если его нет
	s.mu.RLock()
	stream := s.repo.GetStream(streamID)
	s.mu.RUnlock()

	if stream == nil {
		s.logger.Info("Auto-creating stream",
			zap.String("stream_id", streamID),
			zap.String("client_id", clientID))

		activeStream := &pb.ActiveStream{
			StreamId:    streamID,
			ClientId:    clientID,
			UserName:    userName,
			CameraName:  "auto_created",
			IsRecording: true,
			IsStreaming: true,
		}

		s.mu.Lock()
		s.repo.SaveStream(streamID, activeStream)
		s.mu.Unlock()
	}

	// Обновляем статистику
	stats := s.repo.UpdateStats(streamID, frame)

	s.logger.Debug("Frame received",
		zap.String("stream_id", streamID),
		zap.String("client_id", clientID),
		zap.Int64("frame_size", int64(len(frame.FrameData))),
		zap.Int64("total_frames", stats.GetFramesReceived()),
		zap.Int64("total_bytes", stats.GetBytesReceived()))

	return &pb.ApiResponse{
		Status:    "ok",
		Message:   "Frame received",
		Timestamp: time.Now().Unix(),
		Metadata: map[string]string{
			"stream_id":       streamID,
			"client_id":       clientID,
			"frame_id":        frame.FrameId,
			"frames_received": fmt.Sprintf("%d", stats.GetFramesReceived()),
			"bytes_received":  fmt.Sprintf("%d", stats.GetBytesReceived()),
			"source":          "video_service",
		},
	}, nil
}

// StopStream - остановка стрима
func (s *VideoStreamServiceImpl) StopStream(
	ctx context.Context,
	req *pb.StopStreamRequest,
) (*pb.ApiResponse, error) {
	s.logger.Info("Stopping stream",
		zap.String("stream_id", req.StreamId),
		zap.String("client_id", req.ClientId))

	s.repo.RemoveStream(req.StreamId)

	return &pb.ApiResponse{
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
