package gateway

import (
	"api-gateway/proto"
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/cors"

	"api-gateway/internal/config"
)

// APIGateway основной шлюз
type APIGateway struct {
	config     *config.Config
	clientMgr  *ClientManager
	services   *ServiceRegistry
	stats      *GatewayStats
	statsMutex sync.RWMutex

	// HTTP сервер
	httpServer *http.Server
	wsUpgrader websocket.Upgrader

	// Каналы для обработки сообщений
	videoChan   chan *proto.VideoFrame
	controlChan chan *ControlMessage

	// Контекст для graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type GatewayStats struct {
	StartTime      time.Time
	TotalRequests  int64
	TotalFrames    int64
	ActiveClients  int32
	BytesProcessed int64
	ErrorCount     int64
	ServiceHealth  map[string]bool
}

type ControlMessage struct {
	Type      string
	ClientID  string
	Channel   string
	Data      interface{}
	Timestamp time.Time
}

// NewAPIGateway создает новый экземпляр шлюза
func NewAPIGateway(cfg *config.Config) (*APIGateway, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Создаем менеджер клиентов
	clientMgr := NewClientManager()

	// Создаем реестр сервисов
	serviceRegistry := NewServiceRegistry(cfg)

	gateway := &APIGateway{
		config:    cfg,
		clientMgr: clientMgr,
		services:  serviceRegistry,
		stats: &GatewayStats{
			StartTime:     time.Now(),
			ServiceHealth: make(map[string]bool),
		},
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				if !cfg.Security.EnableCORS {
					return true
				}
				origin := r.Header.Get("Origin")
				for _, allowed := range cfg.Security.AllowedOrigins {
					if allowed == "*" || allowed == origin {
						return true
					}
				}
				return false
			},
		},
		videoChan:   make(chan *proto.VideoFrame, cfg.Gateway.BufferSize),
		controlChan: make(chan *ControlMessage, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Запускаем обработчики сообщений
	gateway.startMessageProcessors()

	// Запускаем фоновые задачи
	gateway.startBackgroundTasks()

	return gateway, nil
}

// Start запускает API Gateway
func (g *APIGateway) Start() error {
	// Создаем HTTP роутер
	mux := http.NewServeMux()

	// Настраиваем обработчики
	g.setupHTTPHandlers(mux)

	// Настраиваем CORS
	var handler http.Handler = mux
	if g.config.Security.EnableCORS {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins:   g.config.Security.AllowedOrigins,
			AllowedMethods:   g.config.Security.AllowedMethods,
			AllowedHeaders:   g.config.Security.AllowedHeaders,
			AllowCredentials: true,
			MaxAge:           86400,
		})
		handler = corsHandler.Handler(mux)
	}

	// Создаем HTTP сервер
	g.httpServer = &http.Server{
		Addr:         g.config.Server.HTTPPort,
		Handler:      handler,
		ReadTimeout:  g.config.GetReadTimeout(),
		WriteTimeout: g.config.GetWriteTimeout(),
		IdleTimeout:  g.config.GetIdleTimeout(),
	}

	// Запускаем сервер в горутине
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		log.Printf("Starting HTTP server on %s", g.config.Server.HTTPPort)

		var err error
		if g.config.Server.EnableTLS && g.config.Server.TLSCert != "" && g.config.Server.TLSKey != "" {
			err = g.httpServer.ListenAndServeTLS(g.config.Server.TLSCert, g.config.Server.TLSKey)
		} else {
			err = g.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Printf("API Gateway started successfully")
	log.Printf("HTTP: %s, WebSocket: %s", g.config.Server.HTTPPort, g.config.Server.WebSocketPort)

	return nil
}

// Stop останавливает API Gateway
func (g *APIGateway) Stop() {
	log.Println("Shutting down API Gateway...")

	// Отменяем контекст
	g.cancel()

	// Останавливаем HTTP сервер
	if g.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := g.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	// Закрываем все соединения
	g.clientMgr.CloseAll()
	g.services.Close()

	// Закрываем каналы
	close(g.videoChan)
	close(g.controlChan)

	// Ждем завершения всех горутин
	g.wg.Wait()

	log.Println("API Gateway stopped gracefully")
}

// startMessageProcessors запускает обработчики сообщений
func (g *APIGateway) startMessageProcessors() {
	// Обработчик видеофреймов
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.processVideoFrames()
	}()

	// Обработчик контрольных сообщений
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.processControlMessages()
	}()
}

// processVideoFrames обрабатывает входящие видеофреймы
func (g *APIGateway) processVideoFrames() {
	for {
		select {
		case frame := <-g.videoChan:
			g.handleVideoFrame(frame)
		case <-g.ctx.Done():
			return
		}
	}
}

// processControlMessages обрабатывает контрольные сообщения
func (g *APIGateway) processControlMessages() {
	for {
		select {
		case msg := <-g.controlChan:
			g.handleControlMessage(msg)
		case <-g.ctx.Done():
			return
		}
	}
}

// handleVideoFrame обрабатывает видеофрейм
func (g *APIGateway) handleVideoFrame(frame *proto.VideoFrame) {
	g.statsMutex.Lock()
	g.stats.TotalFrames++
	g.stats.BytesProcessed += int64(len(frame.FrameData))
	g.statsMutex.Unlock()

	// Маршрутизируем фрейм в сервисы
	g.routeFrameToServices(frame)

	// Рассылаем фрейм подписанным клиентам
	g.broadcastFrameToClients(frame)
}

// routeFrameToServices маршрутизирует фрейм в сервисы
func (g *APIGateway) routeFrameToServices(frame *proto.VideoFrame) {
	services := g.services.GetServicesForFrame(frame)

	for _, service := range services {
		g.wg.Add(1)
		go func(svc *ServiceEndpoint) {
			defer g.wg.Done()
			g.sendToService(svc, frame)
		}(service)
	}
}

// sendToService отправляет фрейм в сервис
func (g *APIGateway) sendToService(service *ServiceEndpoint, frame *proto.VideoFrame) {
	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	defer cancel()

	g.services.SendToService(ctx, service, frame)
}

// broadcastFrameToClients рассылает фрейм клиентам
func (g *APIGateway) broadcastFrameToClients(frame *proto.VideoFrame) {
	clients := g.clientMgr.GetClientsByChannel(frame.CameraID)

	for _, client := range clients {
		g.wg.Add(1)
		go func(cl *ClientInfo) {
			defer g.wg.Done()
			g.sendFrameToClient(cl, frame)
		}(client)
	}
}

// sendFrameToClient отправляет фрейм конкретному клиенту
func (g *APIGateway) sendFrameToClient(client *ClientInfo, frame *proto.VideoFrame) {
	select {
	case client.SendChan <- frame:
		// Успешно отправлено
	default:
		// Канал полон
		log.Printf("Client %s channel full, dropping frame", client.ID)
	}
}

// handleControlMessage обрабатывает контрольные сообщения
func (g *APIGateway) handleControlMessage(msg *ControlMessage) {
	switch msg.Type {
	case "client_connect":
		g.handleClientConnect(msg)
	case "client_disconnect":
		g.handleClientDisconnect(msg)
	case "subscribe":
		g.handleSubscribe(msg)
	case "unsubscribe":
		g.handleUnsubscribe(msg)
	case "service_health":
		g.handleServiceHealth(msg)
	}
}

// startBackgroundTasks запускает фоновые задачи
func (g *APIGateway) startBackgroundTasks() {
	// Очистка неактивных клиентов
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		ticker := time.NewTicker(g.config.GetHealthCheckInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				g.clientMgr.CleanupInactiveClients(g.config.GetSessionTimeout())
			case <-g.ctx.Done():
				return
			}
		}
	}()

	// Проверка здоровья сервисов
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				g.services.CheckHealth()
			case <-g.ctx.Done():
				return
			}
		}
	}()
}

// HandleVideoFrame добавляет видеофрейм в очередь обработки
func (g *APIGateway) HandleVideoFrame(frame *proto.VideoFrame) {
	select {
	case g.videoChan <- frame:
		// Успешно добавлено
	default:
		// Очередь переполнена
		log.Printf("Video channel full, dropping frame")
		g.statsMutex.Lock()
		g.stats.ErrorCount++
		g.statsMutex.Unlock()
	}
}
