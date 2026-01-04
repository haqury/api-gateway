package gateway

import (
	"api-gateway/proto"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// setupHTTPHandlers настраивает HTTP обработчики
func (g *APIGateway) setupHTTPHandlers(mux *http.ServeMux) {
	// API эндпоинты
	mux.HandleFunc("/api/v1/video/stream", g.handleVideoStream)
	mux.HandleFunc("/api/v1/video/info", g.handleVideoInfo)
	mux.HandleFunc("/api/v1/clients", g.handleClients)
	mux.HandleFunc("/api/v1/stats", g.handleStats)
	mux.HandleFunc("/api/v1/health", g.handleHealth)

	// WebSocket
	mux.HandleFunc("/ws/video", g.handleWebSocketVideo)
	mux.HandleFunc("/ws/control", g.handleWebSocketControl)

	// Статическая страница
	mux.HandleFunc("/", g.handleRoot)
}

// handleRoot обрабатывает корневой путь
func (g *APIGateway) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Video API Gateway</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .card { border: 1px solid #ddd; padding: 20px; margin: 10px 0; border-radius: 5px; }
        button { padding: 10px 20px; background: #007bff; color: white; border: none; border-radius: 5px; cursor: pointer; }
        button:hover { background: #0056b3; }
        pre { background: #f5f5f5; padding: 10px; border-radius: 5px; overflow: auto; }
    </style>
</head>
<body>
    <h1>Video API Gateway</h1>
    
    <div class="card">
        <h3>Status: <span style="color: green;">● Running</span></h3>
        <p>HTTP Port: ` + g.config.Server.HTTPPort + `</p>
        <p>WebSocket Port: ` + g.config.Server.WebSocketPort + `</p>
    </div>
    
    <div class="card">
        <h3>API Endpoints:</h3>
        <ul>
            <li><strong>POST /api/v1/video/stream</strong> - Send video frame</li>
            <li><strong>POST /api/v1/video/info</strong> - Video metadata</li>
            <li><strong>GET /api/v1/clients</strong> - Connected clients</li>
            <li><strong>GET /api/v1/stats</strong> - Gateway statistics</li>
            <li><strong>GET /api/v1/health</strong> - Health check</li>
            <li><strong>GET /ws/video</strong> - WebSocket stream</li>
        </ul>
    </div>
    
    <div class="card">
        <h3>Test Interface:</h3>
        <button onclick="sendTestFrame()">Send Test Frame</button>
        <button onclick="connectWebSocket()">Connect WebSocket</button>
        <button onclick="getStats()">Get Stats</button>
        <div id="output"></div>
    </div>
    
    <script>
        const output = document.getElementById('output');
        
        function sendTestFrame() {
            const frame = {
                frame_id: 'test_' + Date.now(),
                frame_data: btoa('test video data'),
                timestamp: Math.floor(Date.now() / 1000),
                camera_id: 'test-camera-1',
                client_id: 'web-client',
                width: 1920,
                height: 1080,
                format: 'h264'
            };
            
            fetch('/api/v1/video/stream', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(frame)
            })
            .then(r => r.json())
            .then(data => {
                output.innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
            })
            .catch(err => {
                output.innerHTML = 'Error: ' + err;
            });
        }
        
        function getStats() {
            fetch('/api/v1/stats')
            .then(r => r.json())
            .then(data => {
                output.innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
            })
            .catch(err => {
                output.innerHTML = 'Error: ' + err;
            });
        }
        
        function connectWebSocket() {
            const ws = new WebSocket('ws://' + window.location.host + '/ws/video');
            ws.onopen = () => {
                output.innerHTML = 'WebSocket connected';
                ws.send(JSON.stringify({
                    action: 'subscribe',
                    channel: 'test-camera-1'
                }));
            };
            ws.onmessage = (event) => {
                output.innerHTML = 'Received: ' + event.data;
            };
            ws.onerror = (error) => {
                output.innerHTML = 'WebSocket error: ' + error;
            };
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleVideoStream обрабатывает видеострим
func (g *APIGateway) handleVideoStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем размер контента
	if r.ContentLength > int64(g.config.Gateway.MaxFrameSize) {
		http.Error(w, "Frame too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Читаем тело
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим JSON
	var frame proto.VideoFrame
	if err := json.Unmarshal(body, &frame); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if frame.FrameID == "" || frame.CameraID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Устанавливаем ClientID если не указан
	if frame.ClientID == "" {
		frame.ClientID = getIPAddress(r)
	}

	// Обновляем статистику
	g.statsMutex.Lock()
	g.stats.TotalRequests++
	g.statsMutex.Unlock()

	// Обрабатываем фрейм
	g.HandleVideoFrame(&frame)

	// Отправляем ответ
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Frame received for processing",
		"frame_id":  frame.FrameID,
		"timestamp": time.Now().Unix(),
		"services":  len(g.config.Services.VideoProcessing),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleVideoInfo обрабатывает информацию о видео
func (g *APIGateway) handleVideoInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var info struct {
		Filename  string `json:"filename"`
		CameraID  string `json:"camera_id"`
		ClientID  string `json:"client_id"`
		StartTime int64  `json:"start_time"`
		Duration  int    `json:"duration"`
		Size      int64  `json:"size"`
	}

	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Отправляем контрольное сообщение
	g.controlChan <- &ControlMessage{
		Type:      "video_info",
		ClientID:  info.ClientID,
		Channel:   info.CameraID,
		Data:      info,
		Timestamp: time.Now(),
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Video info received",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClients обрабатывает запросы клиентов
func (g *APIGateway) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		clients := g.clientMgr.GetAllClients()

		var clientList []map[string]interface{}
		for _, client := range clients {
			channels := make([]string, 0, len(client.Channels))
			for ch := range client.Channels {
				channels = append(channels, ch)
			}

			clientList = append(clientList, map[string]interface{}{
				"id":            client.ID,
				"connection_id": client.ConnectionID,
				"ip_address":    client.IPAddress,
				"connected_at":  client.ConnectedAt,
				"last_seen":     client.LastSeen,
				"is_active":     client.IsActive,
				"channels":      channels,
			})
		}

		response := map[string]interface{}{
			"status":  "success",
			"clients": clientList,
			"count":   len(clientList),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats обрабатывает статистику
func (g *APIGateway) handleStats(w http.ResponseWriter, r *http.Request) {
	g.statsMutex.RLock()
	stats := *g.stats
	g.statsMutex.RUnlock()

	// Добавляем текущие данные
	stats.ActiveClients = int32(g.clientMgr.GetActiveClientCount())

	response := map[string]interface{}{
		"status": "success",
		"stats": map[string]interface{}{
			"uptime_seconds":  time.Since(stats.StartTime).Seconds(),
			"total_requests":  stats.TotalRequests,
			"total_frames":    stats.TotalFrames,
			"active_clients":  stats.ActiveClients,
			"bytes_processed": stats.BytesProcessed,
			"error_count":     stats.ErrorCount,
			"frame_rate":      stats.FrameRate,
			"services_health": g.services.GetHealthStatus(),
			"queue_size":      len(g.videoChan),
		},
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth обрабатывает проверку здоровья
func (g *APIGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"service":   "api-gateway",
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
		"services":  g.services.GetHealthStatus(),
	}

	// Проверяем критичные компоненты
	if len(g.videoChan) > g.config.Gateway.BufferSize*90/100 {
		health["status"] = "degraded"
		health["warning"] = "High queue load"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleWebSocketVideo обрабатывает WebSocket для видео
func (g *APIGateway) handleWebSocketVideo(w http.ResponseWriter, r *http.Request) {
	conn, err := g.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Регистрируем клиента
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = getIPAddress(r)
	}

	clientInfo, err := g.clientMgr.RegisterClient(clientID, getIPAddress(r), r.UserAgent())
	if err != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Failed to register client"))
		conn.Close()
		return
	}

	// Создаем сессию
	session := &WebSocketSession{
		Conn:       conn,
		ClientInfo: clientInfo,
		SendChan:   clientInfo.SendChan,
		Done:       make(chan struct{}),
	}

	// Запускаем обработку
	go g.handleWebSocketSession(session)

	log.Printf("WebSocket client connected: %s", clientID)
}

// handleWebSocketControl обрабатывает WebSocket для управления
func (g *APIGateway) handleWebSocketControl(w http.ResponseWriter, r *http.Request) {
	conn, err := g.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	go g.handleControlWebSocket(conn)
}

// WebSocketSession представляет WebSocket сессию
type WebSocketSession struct {
	Conn       *websocket.Conn
	ClientInfo *ClientInfo
	SendChan   chan *proto.VideoFrame
	Done       chan struct{}
}

// handleWebSocketSession обрабатывает WebSocket сессию
func (g *APIGateway) handleWebSocketSession(session *WebSocketSession) {
	defer func() {
		session.Conn.Close()
		close(session.Done)
		g.clientMgr.RemoveClient(session.ClientInfo.ConnectionID)
	}()

	// Запускаем чтение и запись
	go g.readWebSocketMessages(session)
	g.writeWebSocketMessages(session)
}

// readWebSocketMessages читает сообщения из WebSocket
func (g *APIGateway) readWebSocketMessages(session *WebSocketSession) {
	for {
		messageType, message, err := session.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		session.ClientInfo.LastSeen = time.Now()

		if messageType == websocket.TextMessage {
			g.handleWebSocketCommand(session, message)
		}
	}
}

// writeWebSocketMessages пишет сообщения в WebSocket
func (g *APIGateway) writeWebSocketMessages(session *WebSocketSession) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case frame, ok := <-session.SendChan:
			if !ok {
				return
			}

			data, err := json.Marshal(frame)
			if err != nil {
				log.Printf("Failed to marshal frame: %v", err)
				continue
			}

			err = session.Conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			// Ping для поддержания соединения
			err := session.Conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}

		case <-session.Done:
			return
		}
	}
}

// handleWebSocketCommand обрабатывает команды WebSocket
func (g *APIGateway) handleWebSocketCommand(session *WebSocketSession, message []byte) {
	var command map[string]interface{}
	if err := json.Unmarshal(message, &command); err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		return
	}

	action, ok := command["action"].(string)
	if !ok {
		return
	}

	switch action {
	case "subscribe":
		if channel, ok := command["channel"].(string); ok {
			g.clientMgr.SubscribeClient(session.ClientInfo.ConnectionID, channel)

			response := map[string]interface{}{
				"action":  "subscribed",
				"channel": channel,
				"time":    time.Now().Unix(),
			}

			jsonResponse, _ := json.Marshal(response)
			session.Conn.WriteMessage(websocket.TextMessage, jsonResponse)
		}

	case "unsubscribe":
		if channel, ok := command["channel"].(string); ok {
			g.clientMgr.UnsubscribeClient(session.ClientInfo.ConnectionID, channel)
		}

	case "ping":
		response := map[string]interface{}{
			"action": "pong",
			"time":   time.Now().Unix(),
		}

		jsonResponse, _ := json.Marshal(response)
		session.Conn.WriteMessage(websocket.TextMessage, jsonResponse)
	}
}

// handleControlWebSocket обрабатывает управляющий WebSocket
func (g *APIGateway) handleControlWebSocket(conn *websocket.Conn) {
	defer conn.Close()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.TextMessage {
			// Обработка управляющих команд
			log.Printf("Control command: %s", message)
		}
	}
}

// Вспомогательная функция для получения IP адреса
func getIPAddress(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip, _, _ = splitHostPort(r.RemoteAddr)
	}
	return ip
}

func splitHostPort(hostport string) (host, port string, err error) {
	// Упрощенная реализация
	parts := strings.Split(hostport, ":")
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return hostport, "", nil
}

// FrameRate рассчитывает FPS
func (s *GatewayStats) FrameRate() float64 {
	elapsed := time.Since(s.StartTime).Seconds()
	if elapsed > 0 {
		return float64(s.TotalFrames) / elapsed
	}
	return 0
}
