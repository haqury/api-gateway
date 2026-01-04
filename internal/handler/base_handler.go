package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type BaseHandler struct {
	logger *zap.Logger
}

func NewBaseHandler(logger *zap.Logger) *BaseHandler {
	return &BaseHandler{logger: logger}
}

// BindProtoJSON - привязка JSON к protobuf сообщению
func (h *BaseHandler) BindProtoJSON(c *gin.Context, msg proto.Message) error {
	body, err := c.GetRawData()
	if err != nil {
		return err
	}

	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	if err := unmarshaler.Unmarshal(body, msg); err != nil {
		return err
	}

	return nil
}

// SuccessResponse - успешный ответ
func (h *BaseHandler) SuccessResponse(c *gin.Context, data interface{}) {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	switch v := data.(type) {
	case proto.Message:
		jsonBytes, err := marshaler.Marshal(v)
		if err != nil {
			h.ErrorResponse(c, http.StatusInternalServerError, "Failed to marshal response", err)
			return
		}
		c.Data(http.StatusOK, "application/json", jsonBytes)
	default:
		c.JSON(http.StatusOK, data)
	}
}

// ErrorResponse - ответ с ошибкой (с поддержкой error)
func (h *BaseHandler) ErrorResponse(c *gin.Context, status int, message string, err error) {
	h.logger.Error(message,
		zap.Error(err),
		zap.Int("status", status),
		zap.String("path", c.Request.URL.Path))

	errorDetails := ""
	if err != nil {
		errorDetails = err.Error()
	}

	c.JSON(status, gin.H{
		"error":   message,
		"details": errorDetails,
	})
}

// SimpleErrorResponse - упрощенный ответ с ошибкой (для обратной совместимости)
func (h *BaseHandler) SimpleErrorResponse(c *gin.Context, code int, message string) {
	h.logger.Warn("API error",
		zap.Int("status", code),
		zap.String("message", message),
		zap.String("path", c.Request.URL.Path))

	c.JSON(code, gin.H{
		"error":   http.StatusText(code),
		"message": message,
	})
}

// ValidationError - ошибка валидации
func (h *BaseHandler) ValidationError(c *gin.Context, field string, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":   "validation_error",
		"field":   field,
		"message": message,
	})
}

// ParseQueryParams - парсинг query параметров
func (h *BaseHandler) ParseQueryParams(c *gin.Context) map[string]string {
	params := make(map[string]string)

	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	return params
}
