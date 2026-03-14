package krabot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// handleWebSocket upgrades HTTP to WebSocket and manages the connection.
func (c *KrabotChannel) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !c.IsRunning() {
		http.Error(w, "channel not running", http.StatusServiceUnavailable)
		return
	}

	// Authenticate
	if !c.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check connection limit
	maxConns := c.config.GetMaxConnections()
	if int(c.connCount.Load()) >= maxConns {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorCF("krabot", "WebSocket upgrade failed", map[string]any{
			"error": err.Error(),
		})
		return
	}

	// Get or create session
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	session := c.getOrCreateSession(sessionID)

	kc := &krabotConn{
		id:        uuid.New().String(),
		conn:      conn,
		sessionID: sessionID,
	}

	// Register connection in session
	session.mu.Lock()
	session.connIDs[kc.id] = true
	session.mu.Unlock()

	c.connections.Store(kc.id, kc)
	c.connCount.Add(1)

	logger.InfoCF("krabot", "Client connected", map[string]any{
		"conn_id":    kc.id,
		"session_id": sessionID,
	})

	// Start read loop
	go c.readLoop(kc)
}

// authenticate checks the Bearer token or query param token.
func (c *KrabotChannel) authenticate(r *http.Request) bool {
	token := c.config.Token
	if token == "" {
		return false
	}

	// Check Bearer token in header (preferred, more secure)
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return after == token
	}

	// Fall back to query parameter (less secure, for development/simple clients)
	queryToken := r.URL.Query().Get("token")
	if queryToken != "" {
		return queryToken == token
	}

	return false
}

// readLoop reads messages from a WebSocket connection.
func (c *KrabotChannel) readLoop(kc *krabotConn) {
	defer func() {
		kc.close()
		c.connections.Delete(kc.id)
		c.connCount.Add(-1)

		// Remove from session
		session, ok := c.sessions.Load(kc.sessionID)
		if ok {
			s := session.(*krabotSession)
			s.mu.Lock()
			delete(s.connIDs, kc.id)
			s.mu.Unlock()
		}

		logger.InfoCF("krabot", "Client disconnected", map[string]any{
			"conn_id":    kc.id,
			"session_id": kc.sessionID,
		})
	}()

	// Set initial read deadline
	_ = kc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	kc.conn.SetPongHandler(func(string) error {
		_ = kc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	go c.pingLoop(kc)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, rawMsg, err := kc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.DebugCF("krabot", "WebSocket read error", map[string]any{
					"conn_id": kc.id,
					"error":   err.Error(),
				})
			}
			return
		}

		_ = kc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg KrabotMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			kc.writeJSON(newError("invalid_message", "failed to parse message"))
			continue
		}

		c.handleMessage(kc, msg)
	}
}

// pingLoop sends periodic ping frames.
func (c *KrabotChannel) pingLoop(kc *krabotConn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if kc.closed.Load() {
				return
			}
			kc.writeMu.Lock()
			err := kc.conn.WriteMessage(websocket.PingMessage, nil)
			kc.writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// handleMessage processes an incoming message.
func (c *KrabotChannel) handleMessage(kc *krabotConn, msg KrabotMessage) {
	switch msg.Type {
	case TypePing:
		pong := newMessage(TypePong, MessagePayload{})
		pong.ID = msg.ID
		kc.writeJSON(pong)

	case TypeMessageSend:
		c.handleMessageSend(kc, msg)

	case TypeMediaSend:
		c.handleMediaSend(kc, msg)

	default:
		errMsg := newError("unknown_type", fmt.Sprintf("unknown message type: %s", msg.Type))
		kc.writeJSON(errMsg)
	}
}

// handleMessageSend processes a message.send from a client.
func (c *KrabotChannel) handleMessageSend(kc *krabotConn, msg KrabotMessage) {
	content := strings.TrimSpace(msg.Payload.Content)
	
	// Check if content is empty and no media
	if content == "" && len(msg.Payload.Media) == 0 {
		kc.writeJSON(newError("empty_content", "message content and media are empty"))
		return
	}

	// Process media URLs if present
	var mediaPaths []string
	for _, media := range msg.Payload.Media {
		// Validate media URL
		if media.URL == "" {
			continue
		}
		// Validate MIME type if whitelist configured
		if !c.config.IsAllowedType(media.ContentType) {
			logger.WarnCF("krabot", "Rejected media type", map[string]any{
				"type": media.ContentType,
			})
			continue
		}
		// Store URL reference for AI processing
		// The AI will download from the signed URL
		mediaPaths = append(mediaPaths, media.URL)
	}

	sessionID := kc.sessionID
	chatID := "krabot:" + sessionID
	senderID := "krabot-user"

	peer := bus.Peer{Kind: "direct", ID: chatID}

	metadata := map[string]string{
		"platform":   "krabot",
		"session_id": sessionID,
		"conn_id":    kc.id,
	}

	logger.DebugCF("krabot", "Received message", map[string]any{
		"session_id": sessionID,
		"content":    truncate(content, 50),
		"media":      len(mediaPaths),
	})

	sender := bus.SenderInfo{
		Platform:    "krabot",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("krabot", senderID),
	}

	if !c.IsAllowedSender(sender) {
		return
	}

	// Pass to AI for processing
	c.HandleMessage(c.ctx, peer, msg.ID, senderID, chatID, content, mediaPaths, metadata, sender)
}

// handleMediaSend processes a media.send from a client (media-only message).
func (c *KrabotChannel) handleMediaSend(kc *krabotConn, msg KrabotMessage) {
	// Must have at least one media item
	if len(msg.Payload.Media) == 0 {
		kc.writeJSON(newError("empty_media", "media.send requires at least one media item"))
		return
	}

	// Process media URLs
	var mediaPaths []string
	for _, media := range msg.Payload.Media {
		// Validate media URL
		if media.URL == "" {
			continue
		}
		// Validate MIME type if whitelist configured
		if !c.config.IsAllowedType(media.ContentType) {
			logger.WarnCF("krabot", "Rejected media type", map[string]any{
				"type": media.ContentType,
			})
			continue
		}
		// Store URL reference for AI processing
		mediaPaths = append(mediaPaths, media.URL)
	}

	if len(mediaPaths) == 0 {
		kc.writeJSON(newError("invalid_media", "no valid media URLs provided"))
		return
	}

	sessionID := kc.sessionID
	chatID := "krabot:" + sessionID
	senderID := "krabot-user"

	peer := bus.Peer{Kind: "direct", ID: chatID}

	metadata := map[string]string{
		"platform":   "krabot",
		"session_id": sessionID,
		"conn_id":    kc.id,
	}

	logger.DebugCF("krabot", "Received media", map[string]any{
		"session_id": sessionID,
		"media":      len(mediaPaths),
	})

	sender := bus.SenderInfo{
		Platform:    "krabot",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("krabot", senderID),
	}

	if !c.IsAllowedSender(sender) {
		return
	}

	// Pass to AI for processing (empty content, media-only)
	c.HandleMessage(c.ctx, peer, msg.ID, senderID, chatID, "", mediaPaths, metadata, sender)
}

// truncate truncates a string to maxLen runes.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
