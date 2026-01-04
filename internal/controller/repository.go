package controller

import (
	"sync"
	"time"

	pb "api-gateway/internal/gen"
	videopb "api-gateway/internal/gen"
)

// ClientRepository - репозиторий для клиентов (in-memory)
type ClientRepository struct {
	clients map[string]*pb.ClientInfo
	mu      sync.RWMutex
}

// NewClientRepository создает новый репозиторий
func NewClientRepository() *ClientRepository {
	return &ClientRepository{
		clients: make(map[string]*pb.ClientInfo),
	}
}

// SaveClient сохраняет клиента
func (r *ClientRepository) SaveClient(client *pb.ClientInfo) {
	if client == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Обновляем last_activity внутри Stats
	if client.Stats == nil {
		client.Stats = &pb.ClientInfo_ClientStats{}
	}
	client.Stats.LastActivity = time.Now().Unix()

	r.clients[client.ClientId] = client
}

// GetClient получает клиента по ID
func (r *ClientRepository) GetClient(clientID string) *pb.ClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.clients[clientID]
}

// RemoveClient удаляет клиента
func (r *ClientRepository) RemoveClient(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.clients, clientID)
}

// GetAllClients возвращает всех клиентов
func (r *ClientRepository) GetAllClients() []*pb.ClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients := make([]*pb.ClientInfo, 0, len(r.clients))
	for _, client := range r.clients {
		clients = append(clients, client)
	}

	return clients
}

// StreamRepository - репозиторий для стримов
type StreamRepository struct {
	streams map[string]*videopb.ActiveStream
	stats   map[string]*videopb.StreamStats
	mu      sync.RWMutex
}

// NewStreamRepository создает новый репозиторий
func NewStreamRepository() *StreamRepository {
	return &StreamRepository{
		streams: make(map[string]*videopb.ActiveStream),
		stats:   make(map[string]*videopb.StreamStats),
	}
}

// SaveStream сохраняет стрим
func (r *StreamRepository) SaveStream(streamID string, stream *videopb.ActiveStream) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.streams[streamID] = stream

	// Инициализируем статистику
	if _, exists := r.stats[streamID]; !exists {
		r.stats[streamID] = &videopb.StreamStats{
			StreamId:       streamID,
			ClientId:       stream.ClientId,
			StartTime:      time.Now().Unix(),
			FramesReceived: 0,
			BytesReceived:  0,
			AverageFps:     0.0,
			CurrentFps:     0.0,
			Width:          1920,
			Height:         1080,
			Codec:          "H.264",
			IsRecording:    stream.IsRecording,
			IsStreaming:    stream.IsStreaming,
		}
	}
}

// UpdateStats обновляет статистику с передачей кадра
func (r *StreamRepository) UpdateStats(streamID string, frame *videopb.VideoFrame) *videopb.StreamStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, exists := r.stats[streamID]
	if !exists {
		return nil
	}

	stats.FramesReceived++
	if frame != nil {
		// Добавляем реальный размер кадра
		stats.BytesReceived += int64(len(frame.FrameData))

		// Обновляем размеры кадра
		if frame.Width > 0 {
			stats.Width = frame.Width
		}
		if frame.Height > 0 {
			stats.Height = frame.Height
		}

		// Рассчитываем средний FPS
		now := time.Now().Unix()
		duration := float64(now - stats.StartTime)
		if duration > 0 {
			stats.AverageFps = float32(float64(stats.FramesReceived) / duration)
		}

		// Обновляем длительность
		stats.Duration = now - stats.StartTime
	}

	return stats
}

// GetStats получает статистику
func (r *StreamRepository) GetStats(streamID string) *videopb.StreamStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.stats[streamID]
}

// GetAllStats возвращает всю статистику
func (r *StreamRepository) GetAllStats() []*videopb.StreamStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allStats := make([]*videopb.StreamStats, 0, len(r.stats))
	for _, stats := range r.stats {
		allStats = append(allStats, stats)
	}

	return allStats
}

// GetStream получает стрим по ID
func (r *StreamRepository) GetStream(streamID string) *videopb.ActiveStream {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.streams[streamID]
}

// GetAllStreams возвращает все стримы
func (r *StreamRepository) GetAllStreams() []*videopb.ActiveStream {
	r.mu.RLock()
	defer r.mu.RUnlock()

	streams := make([]*videopb.ActiveStream, 0, len(r.streams))
	for _, stream := range r.streams {
		streams = append(streams, stream)
	}

	return streams
}

// RemoveStream удаляет стрим
func (r *StreamRepository) RemoveStream(streamID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.streams, streamID)
	delete(r.stats, streamID)
}

// GetAllActiveStreams возвращает только активные стримы
func (r *StreamRepository) GetAllActiveStreams() []*videopb.ActiveStream {
	r.mu.RLock()
	defer r.mu.RUnlock()

	activeStreams := make([]*videopb.ActiveStream, 0)
	for _, stream := range r.streams {
		if stream.IsRecording || stream.IsStreaming {
			activeStreams = append(activeStreams, stream)
		}
	}

	return activeStreams
}
