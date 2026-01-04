package gateway

import (
	"api-gateway/proto"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"api-gateway/internal/config"
)

// ServiceRegistry управляет подключениями к сервисам
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string][]*ServiceEndpoint
	config   *config.Config
	client   *http.Client
}

type ServiceEndpoint struct {
	ID        string
	URL       string
	Type      string // "http", "grpc"
	Priority  int
	Healthy   bool
	LastCheck time.Time
	Stats     ServiceStats
}

type ServiceStats struct {
	TotalRequests int64
	SuccessCount  int64
	ErrorCount    int64
	LastResponse  time.Duration
	AverageTime   time.Duration
}

func NewServiceRegistry(cfg *config.Config) *ServiceRegistry {
	registry := &ServiceRegistry{
		services: make(map[string][]*ServiceEndpoint),
		config:   cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Инициализируем сервисы из конфигурации
	registry.initializeServices()

	return registry
}

func (sr *ServiceRegistry) initializeServices() {
	// Видеообработка
	for i, url := range sr.config.Services.VideoProcessing {
		endpoint := &ServiceEndpoint{
			ID:        fmt.Sprintf("video_%d", i),
			URL:       url,
			Type:      "http",
			Priority:  i,
			Healthy:   true,
			LastCheck: time.Now(),
		}
		sr.services["video_processing"] = append(sr.services["video_processing"], endpoint)
	}

	// Аналитика
	for i, url := range sr.config.Services.Analytics {
		endpoint := &ServiceEndpoint{
			ID:        fmt.Sprintf("analytics_%d", i),
			URL:       url,
			Type:      "http",
			Priority:  i,
			Healthy:   true,
			LastCheck: time.Now(),
		}
		sr.services["analytics"] = append(sr.services["analytics"], endpoint)
	}

	// Хранилище
	for i, url := range sr.config.Services.Storage {
		endpoint := &ServiceEndpoint{
			ID:        fmt.Sprintf("storage_%d", i),
			URL:       url,
			Type:      "http",
			Priority:  i,
			Healthy:   true,
			LastCheck: time.Now(),
		}
		sr.services["storage"] = append(sr.services["storage"], endpoint)
	}

	// Уведомления
	for i, url := range sr.config.Services.Notification {
		endpoint := &ServiceEndpoint{
			ID:        fmt.Sprintf("notification_%d", i),
			URL:       url,
			Type:      "http",
			Priority:  i,
			Healthy:   true,
			LastCheck: time.Now(),
		}
		sr.services["notification"] = append(sr.services["notification"], endpoint)
	}
}

// GetServicesForFrame возвращает сервисы для обработки фрейма
func (sr *ServiceRegistry) GetServicesForFrame(frame *proto.VideoFrame) []*ServiceEndpoint {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var endpoints []*ServiceEndpoint

	// Всегда отправляем в видеообработку
	endpoints = append(endpoints, sr.getHealthyServices("video_processing")...)

	// Отправляем в аналитику если включена
	if frame.ClientData != nil && frame.ClientData.Authenticated {
		endpoints = append(endpoints, sr.getHealthyServices("analytics")...)
	}

	// Отправляем в хранилище
	endpoints = append(endpoints, sr.getHealthyServices("storage")...)

	return endpoints
}

// getHealthyServices возвращает только здоровые сервисы
func (sr *ServiceRegistry) getHealthyServices(serviceType string) []*ServiceEndpoint {
	var healthy []*ServiceEndpoint
	for _, endpoint := range sr.services[serviceType] {
		if endpoint.Healthy {
			healthy = append(healthy, endpoint)
		}
	}
	return healthy
}

// SendToService отправляет фрейм в сервис
func (sr *ServiceRegistry) SendToService(ctx context.Context, service *ServiceEndpoint, frame *proto.VideoFrame) error {
	startTime := time.Now()

	// Подготавливаем данные для отправки
	data, err := json.Marshal(frame)
	if err != nil {
		sr.updateServiceStats(service, false, 0)
		return fmt.Errorf("failed to marshal frame: %v", err)
	}

	// Создаем запрос
	req, err := http.NewRequestWithContext(ctx, "POST", service.URL, strings.NewReader(string(data)))
	if err != nil {
		sr.updateServiceStats(service, false, 0)
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Gateway", "video-streaming")
	req.Header.Set("X-Client-ID", frame.ClientID)

	// Отправляем запрос
	resp, err := sr.client.Do(req)
	if err != nil {
		sr.updateServiceStats(service, false, time.Since(startTime))
		service.Healthy = false
		return fmt.Errorf("failed to send to service %s: %v", service.URL, err)
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	// Проверяем статус ответа
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		sr.updateServiceStats(service, true, responseTime)
		service.Healthy = true
		return nil
	} else {
		sr.updateServiceStats(service, false, responseTime)
		service.Healthy = false
		return fmt.Errorf("service %s returned error status: %d", service.URL, resp.StatusCode)
	}
}

// updateServiceStats обновляет статистику сервиса
func (sr *ServiceRegistry) updateServiceStats(service *ServiceEndpoint, success bool, responseTime time.Duration) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	service.Stats.TotalRequests++
	if success {
		service.Stats.SuccessCount++
	} else {
		service.Stats.ErrorCount++
	}

	service.Stats.LastResponse = responseTime

	// Обновляем среднее время
	if service.Stats.SuccessCount > 0 {
		totalTime := service.Stats.AverageTime*time.Duration(service.Stats.SuccessCount-1) + responseTime
		service.Stats.AverageTime = totalTime / time.Duration(service.Stats.SuccessCount)
	}

	service.LastCheck = time.Now()
}

// CheckHealth проверяет здоровье всех сервисов
func (sr *ServiceRegistry) CheckHealth() {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	for serviceType, endpoints := range sr.services {
		for _, endpoint := range endpoints {
			healthy := sr.checkEndpointHealth(endpoint)
			endpoint.Healthy = healthy
			endpoint.LastCheck = time.Now()

			if !healthy {
				log.Printf("Service %s (%s) is unhealthy", endpoint.ID, serviceType)
			}
		}
	}
}

func (sr *ServiceRegistry) checkEndpointHealth(endpoint *ServiceEndpoint) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.URL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := sr.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetHealthStatus возвращает статус здоровья всех сервисов
func (sr *ServiceRegistry) GetHealthStatus() map[string]interface{} {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	status := make(map[string]interface{})

	for serviceType, endpoints := range sr.services {
		typeStatus := make(map[string]interface{})
		for _, endpoint := range endpoints {
			typeStatus[endpoint.ID] = map[string]interface{}{
				"healthy":     endpoint.Healthy,
				"last_check":  endpoint.LastCheck,
				"url":         endpoint.URL,
				"total_reqs":  endpoint.Stats.TotalRequests,
				"success":     endpoint.Stats.SuccessCount,
				"errors":      endpoint.Stats.ErrorCount,
				"avg_time_ms": endpoint.Stats.AverageTime.Milliseconds(),
			}
		}
		status[serviceType] = typeStatus
	}

	return status
}

// Close закрывает все соединения
func (sr *ServiceRegistry) Close() {
	// Для HTTP клиента не нужно явное закрытие
}

// handleClientConnect - заглушка
func (g *APIGateway) handleClientConnect(msg *ControlMessage) {
	log.Printf("Client connect: %s", msg.ClientID)
}

// handleClientDisconnect - заглушка
func (g *APIGateway) handleClientDisconnect(msg *ControlMessage) {
	log.Printf("Client disconnect: %s", msg.ClientID)
}

// handleSubscribe - заглушка
func (g *APIGateway) handleSubscribe(msg *ControlMessage) {
	log.Printf("Subscribe: %s to %s", msg.ClientID, msg.Channel)
}

// handleUnsubscribe - заглушка
func (g *APIGateway) handleUnsubscribe(msg *ControlMessage) {
	log.Printf("Unsubscribe: %s from %s", msg.ClientID, msg.Channel)
}

// handleServiceHealth - заглушка
func (g *APIGateway) handleServiceHealth(msg *ControlMessage) {
	log.Printf("Service health check")
}
