package grpc_server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"api-gateway/internal/controller"
	pb "api-gateway/pkg/gen"

	helpy "github.com/haqury/helpy"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VideoStreamServer реализует gRPC сервер
type VideoStreamServer struct {
	pb.UnimplementedVideoStreamServiceServer
	service *controller.VideoStreamServiceImpl
	logger  *zap.Logger
	streams map[string]*StreamSession
	mu      sync.RWMutex
}

// StreamSession управляет сессией стрима
type StreamSession struct {
	StreamID   string
	ClientID   string
	StartTime  time.Time
	LastFrame  time.Time
	FrameCount int64
	BytesCount int64
	mu         sync.RWMutex
}

// NewVideoStreamServer создает новый gRPC сервер
func NewVideoStreamServer(
	service *controller.VideoStreamServiceImpl,
	logger *zap.Logger,
) *VideoStreamServer {
	return &VideoStreamServer{
		service: service,
		logger:  logger,
		streams: make(map[string]*StreamSession),
	}
}

// StreamVideo - потоковая передача видео (бинарный режим)
func (s *VideoStreamServer) StreamVideo(
	stream pb.VideoStreamService_StreamVideoServer,
) error {
	s.logger.Info("Starting gRPC video stream")

	var session *StreamSession
	var totalBytes int64
	var totalFrames int64
	startTime := time.Now()

	for {
		// Получаем чанк от клиента
		chunk, err := stream.Recv()
		if err == io.EOF {
			if session != nil {
				s.logger.Info("Stream completed",
					zap.String("stream_id", session.StreamID),
					zap.Int64("frames", totalFrames),
					zap.Int64("bytes", totalBytes),
					zap.Duration("duration", time.Since(startTime)))
			}
			return nil
		}
		if err != nil {
			s.logger.Error("Stream receive error", zap.Error(err))
			return status.Error(codes.Internal, err.Error())
		}

		// Создаем сессию при первом чанке
		if session == nil {
			session = &StreamSession{
				StreamID:  chunk.StreamId,
				ClientID:  chunk.ClientId,
				StartTime: time.Now(),
				LastFrame: time.Now(),
			}

			s.mu.Lock()
			s.streams[chunk.StreamId] = session
			s.mu.Unlock()

			s.logger.Info("New gRPC stream session",
				zap.String("stream_id", chunk.StreamId),
				zap.String("client_id", chunk.ClientId))
		}

		// Обновляем статистику сессии
		session.mu.Lock()
		session.FrameCount++
		session.BytesCount += int64(len(chunk.Data))
		session.LastFrame = time.Now()
		session.mu.Unlock()

		totalFrames++
		totalBytes += int64(len(chunk.Data))

		// Логируем каждые 100 кадров
		if totalFrames%100 == 0 {
			s.logger.Debug("Stream progress",
				zap.String("stream_id", chunk.StreamId),
				zap.Int64("frames", totalFrames),
				zap.Int64("bytes", totalBytes),
				zap.Float64("fps", float64(totalFrames)/time.Since(startTime).Seconds()))
		}

		// Обрабатываем кадр через контроллер
		frame := &pb.VideoFrame{
			FrameId:   fmt.Sprintf("grpc_%d", totalFrames),
			FrameData: chunk.Data,
			Timestamp: time.Now().Unix(),
			ClientId:  chunk.ClientId,
			CameraId:  "grpc_stream",
			Width:     1920, // Можно извлечь из метаданных
			Height:    1080,
			Format:    "jpeg",
			Metadata:  chunk.Metadata,
		}

		// Отправляем кадр в общую систему
		s.service.SendFrameInternal(
			chunk.StreamId,
			chunk.ClientId,
			"gRPC Client",
			frame,
		)

		// Отправляем подтверждение клиенту
		ack := &pb.ChunkAck{
			Status:           "ok",
			Message:          "Frame received",
			ReceivedAt:       time.Now().Unix(),
			NextExpected:     int32(totalFrames + 1),
			ProcessingTimeMs: float32(time.Since(startTime).Seconds() * 1000),
		}

		if err := stream.Send(ack); err != nil {
			s.logger.Error("Failed to send ack", zap.Error(err))
			return err
		}
	}
}

// SendFrame - единичный кадр (обратная совместимость)
func (s *VideoStreamServer) SendFrame(
	ctx context.Context,
	req *pb.SendFrameRequest,
) (*helpy.ApiResponse, error) {
	s.logger.Info("gRPC SendFrame called",
		zap.String("stream_id", req.StreamId),
		zap.String("client_id", req.ClientId))

	// Делегируем обработку основному сервису
	return s.service.SendFrame(ctx, req)
}

// StartStream - старт стрима
func (s *VideoStreamServer) StartStream(
	ctx context.Context,
	req *pb.StartStreamRequest,
) (*pb.StartStreamResponse, error) {
	return s.service.StartStream(ctx, req)
}

// StopStream - остановка стрима
func (s *VideoStreamServer) StopStream(
	ctx context.Context,
	req *pb.StopStreamRequest,
) (*helpy.ApiResponse, error) {
	return s.service.StopStream(ctx, req)
}

// GetActiveStreams - получение активных стримов
func (s *VideoStreamServer) GetActiveStreams(
	req *pb.EmptyRequest,
	stream pb.VideoStreamService_GetActiveStreamsServer,
) error {
	// Делегируем основному сервису
	activeStreams := s.service.GetAllActiveStreams()

	for _, as := range activeStreams {
		if err := stream.Send(as); err != nil {
			return err
		}
	}

	return nil
}

// GetStreamStats - получение статистики
func (s *VideoStreamServer) GetStreamStats(
	ctx context.Context,
	req *pb.GetStreamStatsRequest,
) (*pb.StreamStats, error) {
	return s.service.GetStreamStats(ctx, req)
}

// Run запускает gRPC сервер
func (s *VideoStreamServer) Run(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(50*1024*1024), // 50MB для видео
		grpc.MaxSendMsgSize(10*1024*1024), // 10MB
	)

	pb.RegisterVideoStreamServiceServer(grpcServer, s)

	s.logger.Info("Starting gRPC server", zap.String("port", port))

	return grpcServer.Serve(lis)
}
