package proto

// Структуры данных для видео
type VideoFrame struct {
	FrameID    string            `json:"frame_id"`
	FrameData  string            `json:"frame_data"` // base64 encoded
	Timestamp  int64             `json:"timestamp"`
	CameraID   string            `json:"camera_id"`
	ClientID   string            `json:"client_id"`
	Width      int32             `json:"width"`
	Height     int32             `json:"height"`
	Format     string            `json:"format"`
	Metadata   map[string]string `json:"metadata"`
	ClientData *ClientData       `json:"client_data"`
}

type ClientData struct {
	UserID        string            `json:"user_id"`
	SessionID     string            `json:"session_id"`
	Device        string            `json:"device"`
	Location      string            `json:"location"`
	Authenticated bool              `json:"authenticated"`
	Roles         []string          `json:"roles"`
	Metadata      map[string]string `json:"metadata"`
}
