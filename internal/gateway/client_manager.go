package gateway

import (
	"api-gateway/proto"
	"fmt"
	"log"
	"sync"
	"time"
)

// ClientManager управляет информацией о клиентах
type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*ClientInfo
}

type ClientInfo struct {
	ID           string
	ConnectionID string
	IPAddress    string
	UserAgent    string
	ConnectedAt  time.Time
	LastSeen     time.Time
	IsActive     bool
	SendChan     chan *proto.VideoFrame
	Channels     map[string]struct{} // Каналы/комнаты
	ClientData   *ClientData
}

type ClientData struct {
	UserID        string
	SessionID     string
	Device        string
	Location      string
	Authenticated bool
	Roles         []string
	Metadata      map[string]string
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*ClientInfo),
	}
}

// RegisterClient регистрирует нового клиента
func (cm *ClientManager) RegisterClient(clientID, ip, userAgent string) (*ClientInfo, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Генерируем уникальный ID соединения
	connID := fmt.Sprintf("%s-%d", clientID, time.Now().UnixNano())

	client := &ClientInfo{
		ID:           clientID,
		ConnectionID: connID,
		IPAddress:    ip,
		UserAgent:    userAgent,
		ConnectedAt:  time.Now(),
		LastSeen:     time.Now(),
		IsActive:     true,
		SendChan:     make(chan *proto.VideoFrame, 100),
		Channels:     make(map[string]struct{}),
		ClientData: &ClientData{
			SessionID:     connID,
			Authenticated: false,
			Metadata:      make(map[string]string),
		},
	}

	cm.clients[connID] = client

	log.Printf("Client registered: %s (connection: %s)", clientID, connID)
	return client, nil
}

// GetClientInfo возвращает информацию о клиенте
func (cm *ClientManager) GetClientInfo(connID string) (*ClientInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	client, exists := cm.clients[connID]
	return client, exists
}

// GetClientInfoByID возвращает информацию о клиенте по ID
func (cm *ClientManager) GetClientInfoByID(clientID string) (*ClientInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, client := range cm.clients {
		if client.ID == clientID {
			return client, true
		}
	}
	return nil, false
}

// SubscribeClient подписывает клиента на канал
func (cm *ClientManager) SubscribeClient(connID, channel string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[connID]
	if !exists {
		return fmt.Errorf("client not found: %s", connID)
	}

	client.Channels[channel] = struct{}{}
	client.LastSeen = time.Now()

	log.Printf("Client %s subscribed to channel %s", client.ID, channel)
	return nil
}

// UnsubscribeClient отписывает клиента от канала
func (cm *ClientManager) UnsubscribeClient(connID, channel string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[connID]
	if !exists {
		return fmt.Errorf("client not found: %s", connID)
	}

	delete(client.Channels, channel)
	client.LastSeen = time.Now()

	log.Printf("Client %s unsubscribed from channel %s", client.ID, channel)
	return nil
}

// GetClientsByChannel возвращает клиентов в указанном канале
func (cm *ClientManager) GetClientsByChannel(channel string) []*ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var clients []*ClientInfo
	for _, client := range cm.clients {
		if _, subscribed := client.Channels[channel]; subscribed {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetAllClients возвращает всех клиентов
func (cm *ClientManager) GetAllClients() []*ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var clients []*ClientInfo
	for _, client := range cm.clients {
		clients = append(clients, client)
	}
	return clients
}

// RemoveClient удаляет клиента
func (cm *ClientManager) RemoveClient(connID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[connID]
	if !exists {
		return
	}

	close(client.SendChan)
	delete(cm.clients, connID)

	log.Printf("Client removed: %s (connection: %s)", client.ID, connID)
}

// CleanupInactiveClients очищает неактивных клиентов
func (cm *ClientManager) CleanupInactiveClients(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for connID, client := range cm.clients {
		if now.Sub(client.LastSeen) > timeout {
			close(client.SendChan)
			delete(cm.clients, connID)
			log.Printf("Inactive client cleaned up: %s", client.ID)
		}
	}
}

// UpdateClientData обновляет данные клиента
func (cm *ClientManager) UpdateClientData(connID string, data *ClientData) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[connID]
	if !exists {
		return fmt.Errorf("client not found: %s", connID)
	}

	client.ClientData = data
	client.LastSeen = time.Now()

	return nil
}

// CloseAll закрывает все соединения
func (cm *ClientManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for connID, client := range cm.clients {
		close(client.SendChan)
		delete(cm.clients, connID)
		log.Printf("Client disconnected on shutdown: %s", client.ID)
	}
}

// GetActiveClientCount возвращает количество активных клиентов
func (cm *ClientManager) GetActiveClientCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	count := 0
	for _, client := range cm.clients {
		if client.IsActive {
			count++
		}
	}
	return count
}
