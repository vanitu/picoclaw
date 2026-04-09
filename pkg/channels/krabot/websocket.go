package krabot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/media"
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

	// Log incoming message
	logger.DebugCF("krabot", "handleMessageSend: received message", map[string]any{
		"session_id":   kc.sessionID,
		"content_len":  len(msg.Payload.Content),
		"media_count":  len(msg.Payload.Media),
		"message_type": msg.Type,
	})

	// Check if content is empty and no media
	if content == "" && len(msg.Payload.Media) == 0 {
		logger.DebugCF("krabot", "handleMessageSend: empty content and media, rejecting", map[string]any{
			"session_id": kc.sessionID,
		})
		kc.writeJSON(newError("empty_content", "message content and media are empty"))
		return
	}

	// Inspect content for debugging
	logger.DebugCF("krabot", "handleMessageSend: content inspection", map[string]any{
		"content_preview":    truncate(msg.Payload.Content, 200),
		"is_json_array":      strings.HasPrefix(strings.TrimSpace(msg.Payload.Content), "["),
		"contains_image_url": strings.Contains(msg.Payload.Content, "image_url"),
		"contains_base64":    strings.Contains(msg.Payload.Content, "base64,"),
	})

	sessionID := kc.sessionID
	chatID := "krabot:" + sessionID
	senderID := "krabot-user"
	scope := channels.BuildMediaScope("krabot", chatID, msg.ID)

	// Process media URLs: download and store locally (like Telegram)
	var mediaPaths []string
	store := c.GetMediaStore()

	for i, mediaItem := range msg.Payload.Media {
		// Log media item details
		logger.DebugCF("krabot", "handleMessageSend: processing media item", map[string]any{
			"index":          i,
			"media_type":     mediaItem.Type,
			"content_type":   mediaItem.ContentType,
			"filename":       mediaItem.Filename,
			"url_preview":    truncate(mediaItem.URL, 100),
			"url_is_signed":  strings.Contains(mediaItem.URL, "?") || strings.Contains(mediaItem.URL, "signed"),
			"url_is_base64":  strings.HasPrefix(mediaItem.URL, "data:"),
		})
		// Validate media URL
		if mediaItem.URL == "" {
			continue
		}
		// Validate MIME type if whitelist configured
		if !c.config.IsAllowedType(mediaItem.ContentType) {
			logger.WarnCF("krabot", "Rejected media type", map[string]any{
				"type": mediaItem.ContentType,
			})
			continue
		}

		// Download file from signed URL
		logger.DebugCF("krabot", "handleMessageSend: downloading media from URL", map[string]any{
			"index":       i,
			"url_preview": truncate(mediaItem.URL, 50),
			"max_size":    c.config.GetMaxFileSize(),
		})

		localPath, err := DownloadFromURL(mediaItem.URL, c.config.GetMaxFileSize())
		if err != nil {
			logger.ErrorCF("krabot", "Failed to download media", map[string]any{
				"url":   mediaItem.URL[:50] + "...",
				"error": err.Error(),
			})
			// Classify error and send detailed response
			dlErr := ClassifyDownloadError(err)
			kc.writeJSON(newErrorWithDetails(
				dlErr.Code,
				dlErr.Message,
				err.Error(),
				dlErr.Recoverable,
			))
			// Fail the message - don't process without media
			return
		}

		logger.DebugCF("krabot", "handleMessageSend: media downloaded successfully", map[string]any{
			"index":       i,
			"local_path":  localPath,
			"file_size":   getFileSize(localPath),
		})

		// Store in MediaStore (same as Telegram)
		if store != nil {
			logger.DebugCF("krabot", "handleMessageSend: storing media in MediaStore", map[string]any{
				"index":        i,
				"local_path":   localPath,
				"filename":     mediaItem.Filename,
				"content_type": mediaItem.ContentType,
				"scope":        scope,
			})

			ref, err := store.Store(localPath, media.MediaMeta{
				Filename:    mediaItem.Filename,
				ContentType: mediaItem.ContentType,
				Source:      "krabot",
			}, scope)
			if err == nil {
				logger.DebugCF("krabot", "handleMessageSend: media stored successfully", map[string]any{
					"index":     i,
					"media_ref": ref,
					"scope":     scope,
				})
				mediaPaths = append(mediaPaths, ref)
				continue
			}
			logger.WarnCF("krabot", "Failed to store media, using temp path", map[string]any{
				"error": err.Error(),
			})
		}
		// Fallback: use local path directly
		mediaPaths = append(mediaPaths, localPath)
	}

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

	// Log final forwarding details
	logger.DebugCF("krabot", "handleMessageSend: forwarding to agent", map[string]any{
		"session_id":     sessionID,
		"chat_id":        chatID,
		"content_preview": truncate(content, 100),
		"media_refs":     mediaPaths,
		"media_count":    len(mediaPaths),
	})

	sender := bus.SenderInfo{
		Platform:    "krabot",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("krabot", senderID),
	}

	if !c.IsAllowedSender(sender) {
		return
	}

	// Pass to AI for processing (media:// refs like Telegram)
	c.HandleMessage(c.ctx, peer, msg.ID, senderID, chatID, content, mediaPaths, metadata, sender)
}

// handleMediaSend processes a media.send from a client (media-only message).
func (c *KrabotChannel) handleMediaSend(kc *krabotConn, msg KrabotMessage) {
	// Log incoming media message
	logger.DebugCF("krabot", "handleMediaSend: received media", map[string]any{
		"session_id":  kc.sessionID,
		"media_count": len(msg.Payload.Media),
		"message_id":  msg.ID,
	})

	// Must have at least one media item
	if len(msg.Payload.Media) == 0 {
		logger.DebugCF("krabot", "handleMediaSend: no media items, rejecting", map[string]any{
			"session_id": kc.sessionID,
		})
		kc.writeJSON(newError("empty_media", "media.send requires at least one media item"))
		return
	}

	sessionID := kc.sessionID
	chatID := "krabot:" + sessionID
	senderID := "krabot-user"
	scope := channels.BuildMediaScope("krabot", chatID, msg.ID)

	// Process media URLs: download and store locally (like Telegram)
	var mediaPaths []string
	store := c.GetMediaStore()

	for i, mediaItem := range msg.Payload.Media {
		// Log media item details
		logger.DebugCF("krabot", "handleMediaSend: processing media item", map[string]any{
			"index":         i,
			"media_type":    mediaItem.Type,
			"content_type":  mediaItem.ContentType,
			"filename":      mediaItem.Filename,
			"url_preview":   truncate(mediaItem.URL, 100),
			"url_is_signed": strings.Contains(mediaItem.URL, "?") || strings.Contains(mediaItem.URL, "signed"),
			"url_is_base64": strings.HasPrefix(mediaItem.URL, "data:"),
		})
		// Validate media URL
		if mediaItem.URL == "" {
			continue
		}
		// Validate MIME type if whitelist configured
		if !c.config.IsAllowedType(mediaItem.ContentType) {
			logger.WarnCF("krabot", "Rejected media type", map[string]any{
				"type": mediaItem.ContentType,
			})
			continue
		}

		// Download file from signed URL
		logger.DebugCF("krabot", "handleMediaSend: downloading media from URL", map[string]any{
			"index":       i,
			"url_preview": truncate(mediaItem.URL, 50),
			"max_size":    c.config.GetMaxFileSize(),
		})

		localPath, err := DownloadFromURL(mediaItem.URL, c.config.GetMaxFileSize())
		if err != nil {
			logger.ErrorCF("krabot", "Failed to download media", map[string]any{
				"url":   mediaItem.URL[:50] + "...",
				"error": err.Error(),
			})
			// Classify error and send detailed response
			dlErr := ClassifyDownloadError(err)
			kc.writeJSON(newErrorWithDetails(
				dlErr.Code,
				dlErr.Message,
				err.Error(),
				dlErr.Recoverable,
			))
			// Fail the media send - don't process without media
			return
		}

		logger.DebugCF("krabot", "handleMediaSend: media downloaded successfully", map[string]any{
			"index":      i,
			"local_path": localPath,
			"file_size":  getFileSize(localPath),
		})

		// Store in MediaStore (same as Telegram)
		if store != nil {
			logger.DebugCF("krabot", "handleMediaSend: storing media in MediaStore", map[string]any{
				"index":        i,
				"local_path":   localPath,
				"filename":     mediaItem.Filename,
				"content_type": mediaItem.ContentType,
				"scope":        scope,
			})

			ref, err := store.Store(localPath, media.MediaMeta{
				Filename:    mediaItem.Filename,
				ContentType: mediaItem.ContentType,
				Source:      "krabot",
			}, scope)
			if err == nil {
				logger.DebugCF("krabot", "handleMediaSend: media stored successfully", map[string]any{
					"index":     i,
					"media_ref": ref,
					"scope":     scope,
				})
				mediaPaths = append(mediaPaths, ref)
				continue
			}
			logger.WarnCF("krabot", "Failed to store media, using temp path", map[string]any{
				"error": err.Error(),
			})
		}
		// Fallback: use local path directly
		mediaPaths = append(mediaPaths, localPath)
	}

	if len(mediaPaths) == 0 {
		logger.DebugCF("krabot", "handleMediaSend: no valid media after processing", map[string]any{
			"session_id": sessionID,
		})
		kc.writeJSON(newError("invalid_media", "no valid media URLs provided"))
		return
	}

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

	// Log final forwarding details
	logger.DebugCF("krabot", "handleMediaSend: forwarding to agent", map[string]any{
		"session_id":  sessionID,
		"chat_id":     chatID,
		"media_refs":  mediaPaths,
		"media_count": len(mediaPaths),
	})

	sender := bus.SenderInfo{
		Platform:    "krabot",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("krabot", senderID),
	}

	if !c.IsAllowedSender(sender) {
		return
	}

	// Pass to AI for processing (media:// refs like Telegram)
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

// getFileSize returns the size of a file in bytes, or 0 if the file doesn't exist.
func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
