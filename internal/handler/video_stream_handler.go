package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"api-gateway/internal/controller"
	gen "api-gateway/internal/gen"
)

// VideoStreamHandler обрабатывает HTTP запросы для видеостримов
type VideoStreamHandler struct {
	logger  *zap.Logger
	service *controller.VideoStreamServiceImpl
}

// NewVideoStreamHandler создает новый хендлер
func NewVideoStreamHandler(
	logger *zap.Logger,
	service *controller.VideoStreamServiceImpl,
) *VideoStreamHandler {
	return &VideoStreamHandler{
		logger:  logger,
		service: service,
	}
}

// RegisterRoutes регистрирует маршруты
func (h *VideoStreamHandler) RegisterRoutes(router *gin.RouterGroup) {
	video := router.Group("/video")
	{
		video.POST("/start", h.StartStream)
		video.POST("/frame", h.SendFrame)
		video.POST("/stop", h.StopStream)
		video.GET("/active", h.GetActiveStreams)
		video.GET("/stats/:client_id", h.GetStreamStats)
		video.GET("/client/:client_id/streams", h.GetClientStreams)
		video.GET("/stream/:stream_id", h.GetStreamInfo)
		video.GET("/all-stats", h.GetAllStats)
	}
}

// StartStream обрабатывает начало стрима
func (h *VideoStreamHandler) StartStream(c *gin.Context) {
	var req gen.StartStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(400, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Устанавливаем значения по умолчанию
	if req.ClientId == "" {
		req.ClientId = fmt.Sprintf("client_%d", time.Now().Unix())
	}
	if req.UserId == "" {
		req.UserId = req.ClientId
	}
	if req.CameraName == "" {
		req.CameraName = "default_camera"
	}
	if req.Filename == "" {
		req.Filename = fmt.Sprintf("stream_%s_%d.mp4", req.ClientId, time.Now().Unix())
	}

	h.logger.Info("Starting stream",
		zap.String("client_id", req.ClientId),
		zap.String("camera", req.CameraName))

	// Вызываем сервис
	response, err := h.service.StartStream(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to start stream", zap.Error(err))
		c.JSON(500, gin.H{
			"error":   "Internal server error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":    "ok",
		"stream_id": response.StreamId,
		"message":   response.Message,
		"timestamp": time.Now().Unix(),
		"details": gin.H{
			"client_id":   req.ClientId,
			"user_id":     req.UserId,
			"camera_name": req.CameraName,
			"filename":    req.Filename,
		},
	})
}

// SendFrame обрабатывает отправку видео кадра
func (h *VideoStreamHandler) SendFrame(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")

	// Определяем формат запроса
	if strings.Contains(contentType, "multipart/form-data") {
		h.handleMultipartFrame(c)
	} else {
		h.handleJSONFrame(c)
	}
}

// handleMultipartFrame обрабатывает multipart запрос с бинарными данными
func (h *VideoStreamHandler) handleMultipartFrame(c *gin.Context) {
	// Получаем файл
	file, header, err := c.Request.FormFile("frame")
	if err != nil {
		h.logger.Error("No frame file in multipart", zap.Error(err))
		c.JSON(400, gin.H{
			"error":   "No frame file",
			"message": "Please include 'frame' file in multipart form",
		})
		return
	}
	defer file.Close()

	// Читаем данные
	frameData, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error("Failed to read frame data", zap.Error(err))
		c.JSON(500, gin.H{
			"error":   "Failed to read frame",
			"message": err.Error(),
		})
		return
	}

	// Получаем метаданные
	metadataStr := c.PostForm("metadata")
	var metadata map[string]interface{}
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			h.logger.Warn("Failed to parse metadata", zap.Error(err))
		}
	}

	// Извлекаем параметры
	streamID := getStringFromMap(metadata, "stream_id", "")
	clientID := getStringFromMap(metadata, "client_id", "")
	userName := getStringFromMap(metadata, "user_name", "multipart_client")
	width := getIntFromMap(metadata, "width", 1920)
	height := getIntFromMap(metadata, "height", 1080)

	// Автогенерация stream_id если не указан
	if streamID == "" {
		if clientID == "" {
			clientID = fmt.Sprintf("multipart_%d", time.Now().Unix())
		}
		streamID = fmt.Sprintf("stream_%s_%d", clientID, time.Now().UnixNano())
		h.logger.Info("Auto-generated stream_id",
			zap.String("stream_id", streamID),
			zap.String("client_id", clientID))
	}

	// Создаем frame
	frame := &gen.VideoFrame{
		FrameId:   fmt.Sprintf("frame_%d", time.Now().UnixNano()),
		FrameData: frameData,
		Timestamp: time.Now().Unix(),
		ClientId:  clientID,
		CameraId:  "multipart_stream",
		Width:     int32(width),
		Height:    int32(height),
		Format:    header.Header.Get("Content-Type"),
	}

	// Обрабатываем кадр
	response, err := h.service.SendFrameInternal(streamID, clientID, userName, frame)
	if err != nil {
		h.logger.Error("Failed to process frame", zap.Error(err))
		c.JSON(500, gin.H{
			"error":   "Failed to process frame",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":     response.Status,
		"message":    response.Message,
		"timestamp":  response.Timestamp,
		"metadata":   response.Metadata,
		"format":     "multipart",
		"frame_size": len(frameData),
		"stream_id":  streamID,
	})
}

// handleJSONFrame обрабатывает JSON запрос (обратная совместимость)
func (h *VideoStreamHandler) handleJSONFrame(c *gin.Context) {
	var req struct {
		StreamID string                 `json:"stream_id"`
		ClientID string                 `json:"client_id"`
		UserName string                 `json:"user_name"`
		Frame    map[string]interface{} `json:"frame"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid JSON request", zap.Error(err))
		c.JSON(400, gin.H{
			"error":   "Invalid JSON",
			"message": err.Error(),
		})
		return
	}

	// Автогенерация stream_id если не указан
	if req.StreamID == "" {
		if req.ClientID == "" {
			req.ClientID = fmt.Sprintf("json_%d", time.Now().Unix())
		}
		req.StreamID = fmt.Sprintf("stream_%s_%d", req.ClientID, time.Now().UnixNano())
		h.logger.Info("Auto-generated stream_id",
			zap.String("stream_id", req.StreamID),
			zap.String("client_id", req.ClientID))
	}

	if req.UserName == "" {
		req.UserName = req.ClientID
	}

	// Извлекаем данные кадра
	frameData, ok := req.Frame["frame_data"].(string)
	if !ok {
		c.JSON(400, gin.H{
			"error":   "Invalid frame data",
			"message": "frame.frame_data is required and must be base64 string",
		})
		return
	}

	// Преобразуем base64 в байты (если нужно)
	// В прото файле уже указано что это base64, так что предполагаем что это строка
	frame := &gen.VideoFrame{
		FrameId:   fmt.Sprintf("frame_%d", time.Now().UnixNano()),
		FrameData: []byte(frameData), // Здесь можно добавить base64 decoding
		Timestamp: getInt64FromMap(req.Frame, "timestamp", time.Now().Unix()),
		ClientId:  req.ClientID,
		CameraId:  getStringFromMap(req.Frame, "camera_id", "json_camera"),
		Width:     int32(getIntFromMap(req.Frame, "width", 1920)),
		Height:    int32(getIntFromMap(req.Frame, "height", 1080)),
		Format:    getStringFromMap(req.Frame, "format", "jpeg"),
	}

	// Обрабатываем кадр
	response, err := h.service.SendFrameInternal(req.StreamID, req.ClientID, req.UserName, frame)
	if err != nil {
		h.logger.Error("Failed to process frame", zap.Error(err))
		c.JSON(500, gin.H{
			"error":   "Failed to process frame",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":     response.Status,
		"message":    response.Message,
		"timestamp":  response.Timestamp,
		"metadata":   response.Metadata,
		"format":     "json_base64",
		"frame_size": len(frameData),
		"stream_id":  req.StreamID,
	})
}

// StopStream обрабатывает остановку стрима
func (h *VideoStreamHandler) StopStream(c *gin.Context) {
	var req struct {
		StreamID string `json:"stream_id"`
		ClientID string `json:"client_id"`
		Filename string `json:"filename"`
		EndTime  int64  `json:"end_time"`
		FileSize int64  `json:"file_size"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(400, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	if req.EndTime == 0 {
		req.EndTime = time.Now().Unix()
	}

	stopReq := &gen.StopStreamRequest{
		StreamId: req.StreamID,
		ClientId: req.ClientID,
		Filename: req.Filename,
		EndTime:  req.EndTime,
		FileSize: req.FileSize,
	}

	response, err := h.service.StopStream(c.Request.Context(), stopReq)
	if err != nil {
		h.logger.Error("Failed to stop stream", zap.Error(err))
		c.JSON(500, gin.H{
			"error":   "Internal server error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":    response.Status,
		"message":   response.Message,
		"timestamp": response.Timestamp,
		"metadata":  response.Metadata,
	})
}

// GetActiveStreams возвращает активные стримы
func (h *VideoStreamHandler) GetActiveStreams(c *gin.Context) {
	activeStreams := h.service.GetAllActiveStreams()

	streams := make([]gin.H, 0, len(activeStreams))
	for _, stream := range activeStreams {
		streams = append(streams, gin.H{
			"stream_id":    stream.StreamId,
			"client_id":    stream.ClientId,
			"user_name":    stream.UserName,
			"camera_name":  stream.CameraName,
			"is_recording": stream.IsRecording,
			"is_streaming": stream.IsStreaming,
		})
	}

	c.JSON(200, gin.H{
		"status":         "ok",
		"active_streams": len(streams),
		"streams":        streams,
		"timestamp":      time.Now().Unix(),
	})
}

// GetStreamStats возвращает статистику стрима
func (h *VideoStreamHandler) GetStreamStats(c *gin.Context) {
	clientID := c.Param("client_id")

	// Получаем все стримы клиента
	clientStreams := h.service.GetStreamsByClient(clientID)

	stats := make([]gin.H, 0, len(clientStreams))
	for _, stream := range clientStreams {
		streamStats, err := h.service.GetStreamStats(c.Request.Context(), &gen.GetStreamStatsRequest{
			StreamId: stream.StreamId,
			ClientId: clientID,
		})

		if err == nil {
			stats = append(stats, gin.H{
				"stream_id":       streamStats.StreamId,
				"client_id":       streamStats.ClientId,
				"start_time":      streamStats.StartTime,
				"duration":        streamStats.Duration,
				"frames_received": streamStats.FramesReceived,
				"bytes_received":  streamStats.BytesReceived,
				"average_fps":     streamStats.AverageFps,
				"current_fps":     streamStats.CurrentFps,
				"width":           streamStats.Width,
				"height":          streamStats.Height,
				"codec":           streamStats.Codec,
				"is_recording":    streamStats.IsRecording,
				"is_streaming":    streamStats.IsStreaming,
			})
		}
	}

	c.JSON(200, gin.H{
		"status":    "ok",
		"client_id": clientID,
		"stats":     stats,
		"timestamp": time.Now().Unix(),
	})
}

// GetClientStreams возвращает стримы клиента
func (h *VideoStreamHandler) GetClientStreams(c *gin.Context) {
	clientID := c.Param("client_id")

	streams := h.service.GetStreamsByClient(clientID)

	result := make([]gin.H, 0, len(streams))
	for _, stream := range streams {
		result = append(result, gin.H{
			"stream_id":    stream.StreamId,
			"client_id":    stream.ClientId,
			"user_name":    stream.UserName,
			"camera_name":  stream.CameraName,
			"is_recording": stream.IsRecording,
			"is_streaming": stream.IsStreaming,
		})
	}

	c.JSON(200, gin.H{
		"status":    "ok",
		"client_id": clientID,
		"count":     len(result),
		"streams":   result,
		"timestamp": time.Now().Unix(),
	})
}

// GetStreamInfo возвращает информацию о конкретном стриме
func (h *VideoStreamHandler) GetStreamInfo(c *gin.Context) {
	streamID := c.Param("stream_id")

	// Здесь можно добавить логику получения конкретного стрима
	// Пока возвращаем заглушку
	c.JSON(200, gin.H{
		"status":    "ok",
		"stream_id": streamID,
		"message":   "Stream info endpoint",
		"timestamp": time.Now().Unix(),
		"endpoints": []string{
			"/api/v1/video/start - Start stream",
			"/api/v1/video/frame - Send frame",
			"/api/v1/video/stop - Stop stream",
			"/api/v1/video/active - Get active streams",
			"/api/v1/video/stats/{client_id} - Get stats",
			"/api/v1/video/client/{client_id}/streams - Get client streams",
		},
	})
}

// GetAllStats возвращает всю статистику
func (h *VideoStreamHandler) GetAllStats(c *gin.Context) {
	allStats := h.service.GetAllStats()

	stats := make([]gin.H, 0, len(allStats))
	totalFrames := int64(0)
	totalBytes := int64(0)

	for _, stat := range allStats {
		totalFrames += stat.FramesReceived
		totalBytes += stat.BytesReceived

		stats = append(stats, gin.H{
			"stream_id":       stat.StreamId,
			"client_id":       stat.ClientId,
			"start_time":      stat.StartTime,
			"duration":        stat.Duration,
			"frames_received": stat.FramesReceived,
			"bytes_received":  stat.BytesReceived,
			"average_fps":     stat.AverageFps,
			"current_fps":     stat.CurrentFps,
		})
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"total_streams": len(stats),
		"total_frames":  totalFrames,
		"total_bytes":   totalBytes,
		"stats":         stats,
		"timestamp":     time.Now().Unix(),
	})
}

// Вспомогательные функции
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if m == nil {
		return defaultValue
	}
	if value, ok := m[key].(string); ok {
		return value
	}
	return defaultValue
}

func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if m == nil {
		return defaultValue
	}
	// Может быть float64 из JSON
	if value, ok := m[key].(float64); ok {
		return int(value)
	}
	if value, ok := m[key].(int); ok {
		return value
	}
	return defaultValue
}

func getInt64FromMap(m map[string]interface{}, key string, defaultValue int64) int64 {
	if m == nil {
		return defaultValue
	}
	if value, ok := m[key].(float64); ok {
		return int64(value)
	}
	if value, ok := m[key].(int64); ok {
		return value
	}
	if value, ok := m[key].(int); ok {
		return int64(value)
	}
	return defaultValue
}
