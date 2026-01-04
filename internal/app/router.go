package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"api-gateway/internal/handler"
)

// NewRouter создает новый роутер с настройкой маршрутов
func NewRouter(
	clientInfoHandler *handler.ClientInfoHandler,
	videoStreamHandler *handler.VideoStreamHandler,
	logger *zap.Logger,
) http.Handler {

	// Режим Gin
	if gin.Mode() == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			logger.Info("HTTP Request",
				zap.String("method", param.Method),
				zap.String("path", param.Path),
				zap.Int("status", param.StatusCode),
				zap.Duration("latency", param.Latency),
				zap.String("client_ip", param.ClientIP),
			)
			return ""
		},
	}))

	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// Статические файлы (если нужно)
	router.Static("/static", "./static")

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "api-gateway",
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		})
	})

	// API v1
	apiV1 := router.Group("/api/v1")
	{
		// Client info endpoints
		clientInfoHandler.RegisterRoutes(apiV1)

		// Video stream endpoints
		videoStreamHandler.RegisterRoutes(apiV1)

		// System endpoints
		apiV1.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "running",
				"timestamp": time.Now().Unix(),
				"endpoints": []string{
					"/api/v1/video/start - POST - Start stream",
					"/api/v1/video/frame - POST - Send frame (auto-creates stream)",
					"/api/v1/video/stop - POST - Stop stream",
					"/api/v1/video/active - GET - Get active streams",
					"/api/v1/video/stats/{client_id} - GET - Get stream stats",
					"/api/v1/video/client/{client_id}/streams - GET - Get client streams",
					"/api/v1/video/stream/{stream_id} - GET - Get stream info",
				},
			})
		})

		// Test endpoints для легкого тестирования
		apiV1.GET("/test/endpoints", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"message": "Available test endpoints",
				"endpoints": map[string]string{
					"health":         "/health",
					"status":         "/api/v1/status",
					"start_stream":   "/api/v1/video/start",
					"send_frame":     "/api/v1/video/frame",
					"stop_stream":    "/api/v1/video/stop",
					"active_streams": "/api/v1/video/active",
					"stream_stats":   "/api/v1/video/stats/{client_id}",
				},
				"example_request": map[string]interface{}{
					"send_frame": map[string]interface{}{
						"method": "POST",
						"url":    "/api/v1/video/frame",
						"body": map[string]interface{}{
							"stream_id": "stream_user_001_123456789",
							"client_id": "user_001",
							"user_name": "Test User",
							"frame": map[string]interface{}{
								"frame_data": "base64_encoded_image_data",
								"timestamp":  time.Now().Unix(),
								"width":      1920,
								"height":     1080,
								"format":     "jpeg",
							},
						},
					},
				},
			})
		})

		// Auto-create test stream endpoint
		apiV1.POST("/test/auto-stream", func(c *gin.Context) {
			var req struct {
				ClientID string `json:"client_id"`
				UserID   string `json:"user_id"`
				Camera   string `json:"camera"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid request",
					"message": err.Error(),
				})
				return
			}

			// Устанавливаем значения по умолчанию
			if req.ClientID == "" {
				req.ClientID = "test_client_" + fmt.Sprintf("%d", time.Now().Unix())
			}
			if req.UserID == "" {
				req.UserID = req.ClientID
			}
			if req.Camera == "" {
				req.Camera = "test_camera"
			}

			// Генерируем stream_id
			streamID := fmt.Sprintf("stream_%s_%d", req.ClientID, time.Now().UnixNano())

			c.JSON(http.StatusOK, gin.H{
				"status":       "ok",
				"message":      "Use this stream_id for testing",
				"instructions": "Send POST request to /api/v1/video/frame with this stream_id",
				"stream_id":    streamID,
				"client_id":    req.ClientID,
				"endpoints": map[string]string{
					"send_frame":      "/api/v1/video/frame",
					"stop_stream":     "/api/v1/video/stop",
					"get_stats":       fmt.Sprintf("/api/v1/video/stats/%s", req.ClientID),
					"get_stream_info": fmt.Sprintf("/api/v1/video/stream/%s", streamID),
				},
				"example_request": map[string]interface{}{
					"url":    "/api/v1/video/frame",
					"method": "POST",
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
					"body": map[string]interface{}{
						"stream_id": streamID,
						"client_id": req.ClientID,
						"user_name": req.UserID,
						"frame": map[string]interface{}{
							"frame_data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
							"timestamp":  time.Now().Unix(),
							"width":      1,
							"height":     1,
							"format":     "png",
						},
					},
				},
			})
		})
	}

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested resource was not found",
			"path":    c.Request.URL.Path,
			"suggestions": []string{
				"Check /health for service status",
				"Check /api/v1/status for API status",
				"Check /api/v1/test/endpoints for available endpoints",
			},
		})
	})

	return router
}

// corsMiddleware настраивает CORS
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods",
			"POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// NewTestRouter создает роутер для тестов
func NewTestRouter(
	clientInfoHandler *handler.ClientInfoHandler,
	videoStreamHandler *handler.VideoStreamHandler,
) *gin.Engine {

	gin.SetMode(gin.TestMode)
	router := gin.New()

	apiV1 := router.Group("/api/v1")
	{
		clientInfoHandler.RegisterRoutes(apiV1)
		videoStreamHandler.RegisterRoutes(apiV1)
	}

	return router
}
