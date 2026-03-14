package a2a

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// WebSocket message types for A2A streaming
type WSMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// handleWebSocket handles WebSocket connections for streaming
func (c *A2AChannel) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !c.config.EnableStreaming {
		http.Error(w, "Streaming not enabled", http.StatusNotImplemented)
		return
	}

	if !c.authenticate(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorCF("a2a", "WebSocket upgrade failed", map[string]any{
			"error": err.Error(),
		})
		return
	}

	clientID := uuid.New().String()

	logger.InfoCF("a2a", "Client connected for streaming", map[string]any{
		"client_id": clientID,
	})

	// Handle connection
	go c.handleWSConnection(conn, clientID)
}

// handleWSConnection manages a WebSocket connection
func (c *A2AChannel) handleWSConnection(conn *websocket.Conn, clientID string) {
	defer func() {
		conn.Close()
		logger.InfoCF("a2a", "Client disconnected from streaming", map[string]any{
			"client_id": clientID,
		})
	}()

	// Set read deadline and pong handler
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	done := make(chan struct{})
	go c.pingLoop(conn, done)

	// Read messages
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.DebugCF("a2a", "WebSocket read error", map[string]any{
					"client_id": clientID,
					"error":     err.Error(),
				})
			}
			close(done)
			return
		}

		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Handle message
		c.handleWSMessage(conn, clientID, msg)
	}
}

// handleWSMessage processes a WebSocket message
func (c *A2AChannel) handleWSMessage(conn *websocket.Conn, clientID string, msg WSMessage) {
	switch msg.Type {
	case "subscribe":
		// Subscribe to task updates
		var payload struct {
			TaskID string `json:"taskId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendWSError(conn, msg.ID, "Invalid payload")
			return
		}

		// Verify task exists
		task, ok := c.GetTask(payload.TaskID)
		if !ok {
			c.sendWSError(conn, msg.ID, "Task not found")
			return
		}

		// Subscribe
		ch := c.subscribeToTask(payload.TaskID, clientID)

		// Send current task state
		resp := StreamResponse{Task: task}
		c.sendWSResponse(conn, msg.ID, "task", resp)

		// Stream updates
		go c.streamUpdates(conn, clientID, payload.TaskID, ch)

	case "unsubscribe":
		// Unsubscribe from task updates
		var payload struct {
			TaskID string `json:"taskId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendWSError(conn, msg.ID, "Invalid payload")
			return
		}

		c.unsubscribeFromTask(payload.TaskID, clientID)
		c.sendWSResponse(conn, msg.ID, "unsubscribed", nil)

	case "ping":
		c.sendWSResponse(conn, msg.ID, "pong", nil)

	default:
		c.sendWSError(conn, msg.ID, "Unknown message type: "+msg.Type)
	}
}

// streamUpdates streams task updates to a WebSocket client
func (c *A2AChannel) streamUpdates(conn *websocket.Conn, clientID, taskID string, ch chan *StreamResponse) {
	defer c.unsubscribeFromTask(taskID, clientID)

	for {
		select {
		case <-c.ctx.Done():
			return

		case resp, ok := <-ch:
			if !ok {
				// Channel closed
				return
			}

			// Send update
			var msgType string
			switch {
			case resp.Task != nil:
				msgType = "task"
			case resp.StatusUpdate != nil:
				msgType = "statusUpdate"
			case resp.ArtifactUpdate != nil:
				msgType = "artifactUpdate"
			}

			if err := c.sendWSResponse(conn, "", msgType, resp); err != nil {
				logger.DebugCF("a2a", "Failed to send WS update", map[string]any{
					"client_id": clientID,
					"error":     err.Error(),
				})
				return
			}

			// Check if task is terminal
			if resp.StatusUpdate != nil && resp.StatusUpdate.Final {
				return
			}
		}
	}
}

// sendWSResponse sends a successful WebSocket response
func (c *A2AChannel) sendWSResponse(conn *websocket.Conn, id, msgType string, data interface{}) error {
	resp := map[string]interface{}{
		"type": msgType,
	}
	if id != "" {
		resp["id"] = id
	}
	if data != nil {
		resp["payload"] = data
	}
	return conn.WriteJSON(resp)
}

// sendWSError sends an error WebSocket response
func (c *A2AChannel) sendWSError(conn *websocket.Conn, id, message string) error {
	resp := map[string]interface{}{
		"type":    "error",
		"payload": map[string]string{"message": message},
	}
	if id != "" {
		resp["id"] = id
	}
	return conn.WriteJSON(resp)
}

// pingLoop sends periodic ping frames
func (c *A2AChannel) pingLoop(conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
