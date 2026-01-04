package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"api-gateway/internal/controller"
	pb "api-gateway/pkg/gen"
)

// ClientInfoHandler обрабатывает запросы к информации о клиентах
type ClientInfoHandler struct {
	logger  *zap.Logger
	service *controller.ClientInfoServiceImpl
}

// NewClientInfoHandler создает новый хендлер
func NewClientInfoHandler(
	logger *zap.Logger,
	service *controller.ClientInfoServiceImpl,
) *ClientInfoHandler {
	return &ClientInfoHandler{
		logger:  logger,
		service: service,
	}
}

// RegisterRoutes регистрирует маршруты
func (h *ClientInfoHandler) RegisterRoutes(router *gin.RouterGroup) {
	clients := router.Group("/clients")
	{
		clients.POST("/connected", h.ClientConnected)
		clients.POST("/disconnected", h.ClientDisconnected)
		clients.PUT("/:client_id", h.UpdateClientInfo)
		clients.GET("/:client_id", h.GetClientInfo)
		clients.GET("/active", h.ListActiveClients)
	}
}

// ClientConnected обрабатывает подключение клиента
func (h *ClientInfoHandler) ClientConnected(c *gin.Context) {
	var req pb.ConnectionEvent
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	resp, err := h.service.ClientConnected(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to process client connection",
			zap.Error(err),
			zap.String("client_id", req.ClientId))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process connection",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ClientDisconnected обрабатывает отключение клиента
func (h *ClientInfoHandler) ClientDisconnected(c *gin.Context) {
	var req pb.ConnectionEvent
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	resp, err := h.service.ClientDisconnected(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to process client disconnection",
			zap.Error(err),
			zap.String("client_id", req.ClientId))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process disconnection",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateClientInfo обновляет информацию о клиенте
func (h *ClientInfoHandler) UpdateClientInfo(c *gin.Context) {
	clientID := c.Param("client_id")

	var req pb.UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Убедимся, что client_id совпадает
	req.ClientId = clientID

	resp, err := h.service.UpdateClientInfo(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to update client info",
			zap.Error(err),
			zap.String("client_id", clientID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update client info",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetClientInfo получает информацию о клиенте
func (h *ClientInfoHandler) GetClientInfo(c *gin.Context) {
	clientID := c.Param("client_id")

	req := &pb.GetClientInfoRequest{
		ClientId: clientID,
	}

	client, err := h.service.GetClientInfo(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get client info",
			zap.Error(err),
			zap.String("client_id", clientID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get client info",
			"message": err.Error(),
		})
		return
	}

	if client == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Client not found",
			"message": "No client found with ID: " + clientID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   client,
	})
}

// ListActiveClients возвращает список активных клиентов
func (h *ClientInfoHandler) ListActiveClients(c *gin.Context) {
	// Получаем параметры пагинации
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.ParseInt(pageStr, 10, 32)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	req := &pb.ListClientsRequest{
		Page:  int32(page),
		Limit: int32(limit),
	}

	resp, err := h.service.ListActiveClients(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to list active clients",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list clients",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   resp.Clients,
		"meta": gin.H{
			"total": resp.Total,
			"page":  page,
			"limit": limit,
		},
	})
}
