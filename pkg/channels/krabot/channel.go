package krabot

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// KrabotChannel implements a human-friendly chat channel with media support.
type KrabotChannel struct {
	*channels.BaseChannel
	config KrabotConfig

	upgrader    websocket.Upgrader
	connections sync.Map // connID → *krabotConn
	connCount   atomic.Int32

	sessions   sync.Map // sessionID → *krabotSession
	sessionMu  sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// krabotConn represents a single WebSocket connection.
type krabotConn struct {
	id        string
	conn      *websocket.Conn
	sessionID string
	writeMu   sync.Mutex
	closed    atomic.Bool
}

// krabotSession represents a chat session.
type krabotSession struct {
	id        string
	connIDs   map[string]bool
	mu        sync.RWMutex
	createdAt int64
}

// writeJSON sends a JSON message to the connection.
func (kc *krabotConn) writeJSON(v any) error {
	if kc.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	kc.writeMu.Lock()
	defer kc.writeMu.Unlock()
	return kc.conn.WriteJSON(v)
}

// close closes the connection.
func (kc *krabotConn) close() {
	if kc.closed.CompareAndSwap(false, true) {
		kc.conn.Close()
	}
}

// NewKrabotChannel creates a new Krabot channel.
func NewKrabotChannel(cfg KrabotConfig, messageBus *bus.MessageBus) (*KrabotChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("krabot token is required")
	}

	base := channels.NewBaseChannel("krabot", cfg, messageBus, cfg.AllowFrom)

	allowOrigins := cfg.AllowOrigins
	checkOrigin := func(r *http.Request) bool {
		if len(allowOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowOrigins {
			if allowed == "*" || allowed == origin {
				return true
			}
		}
		return false
	}

	return &KrabotChannel{
		BaseChannel: base,
		config:      cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin:     checkOrigin,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}, nil
}

// Start implements Channel.
func (c *KrabotChannel) Start(ctx context.Context) error {
	logger.InfoC("krabot", "Starting Krabot channel")
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.SetRunning(true)
	logger.InfoC("krabot", "Krabot channel started")
	return nil
}

// Stop implements Channel.
func (c *KrabotChannel) Stop(ctx context.Context) error {
	logger.InfoC("krabot", "Stopping Krabot channel")
	c.SetRunning(false)

	// Close all connections
	c.connections.Range(func(key, value any) bool {
		if kc, ok := value.(*krabotConn); ok {
			kc.close()
		}
		c.connections.Delete(key)
		return true
	})

	if c.cancel != nil {
		c.cancel()
	}

	logger.InfoC("krabot", "Krabot channel stopped")
	return nil
}

// WebhookPath implements channels.WebhookHandler.
func (c *KrabotChannel) WebhookPath() string { return "/krabot/" }

// ServeHTTP implements http.Handler.
func (c *KrabotChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/krabot")

	switch {
	case path == "/ws" || path == "/ws/":
		c.handleWebSocket(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Send implements Channel - sends a text message.
func (c *KrabotChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	msgID := uuid.New().String()
	outMsg := newMessage(TypeMessageCreate, MessagePayload{
		Content:   msg.Content,
		MessageID: msgID,
		Final:     true,
	})
	return c.broadcastToSession(msg.ChatID, outMsg)
}

// EditMessage implements channels.MessageEditor.
func (c *KrabotChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	outMsg := newMessage(TypeMessageUpdate, MessagePayload{
		Content:   content,
		MessageID: messageID,
		Final:     true,
	})
	return c.broadcastToSession(chatID, outMsg)
}

// StartTyping implements channels.TypingCapable.
func (c *KrabotChannel) StartTyping(ctx context.Context, chatID string) (func(), error) {
	startMsg := newMessage(TypeTypingStart, MessagePayload{})
	if err := c.broadcastToSession(chatID, startMsg); err != nil {
		return func() {}, err
	}
	return func() {
		stopMsg := newMessage(TypeTypingStop, MessagePayload{})
		c.broadcastToSession(chatID, stopMsg)
	}, nil
}

// SendPlaceholder implements channels.PlaceholderCapable.
func (c *KrabotChannel) SendPlaceholder(ctx context.Context, chatID string) (string, error) {
	msgID := uuid.New().String()
	outMsg := newMessage(TypeMessageCreate, MessagePayload{
		Content:   "Thinking... 💭",
		MessageID: msgID,
		Final:     false,
	})

	if err := c.broadcastToSession(chatID, outMsg); err != nil {
		return "", err
	}
	return msgID, nil
}

// sendMediaInternal sends media to clients via signed URLs.
// This is the internal method used by the channel itself.
func (c *KrabotChannel) sendMediaInternal(ctx context.Context, chatID string, media []MediaPart) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	outMsg := newMediaMessage(media)
	return c.broadcastToSession(chatID, outMsg)
}

// SendMedia implements channels.MediaSender interface.
// This is called by the channel manager to deliver OutboundMediaMessage from the agent loop.
func (c *KrabotChannel) SendMedia(ctx context.Context, msg bus.OutboundMediaMessage) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	store := c.GetMediaStore()
	if store == nil {
		return fmt.Errorf("media store not available")
	}

	// Convert bus.MediaPart to krabot.MediaPart, resolving media:// refs
	parts := make([]MediaPart, 0, len(msg.Parts))
	for _, p := range msg.Parts {
		part := MediaPart{
			Type:        p.Type,
			Filename:    p.Filename,
			ContentType: p.ContentType,
		}

		// Resolve media:// refs to local paths
		if strings.HasPrefix(p.Ref, "media://") {
			localPath, meta, err := store.ResolveWithMeta(p.Ref)
			if err != nil {
				logger.WarnCF("krabot", "Failed to resolve media ref", map[string]any{
					"ref":   p.Ref,
					"error": err.Error(),
				})
				continue
			}
			part.LocalPath = localPath
			// Use metadata if fields are empty
			if part.Filename == "" {
				part.Filename = meta.Filename
			}
			if part.ContentType == "" {
				part.ContentType = meta.ContentType
			}
		}

		parts = append(parts, part)
	}

	if len(parts) == 0 {
		return fmt.Errorf("no valid media parts to send")
	}

	// If ActiveStorage is configured, upload files and generate signed URLs
	if c.config.ActiveStorage.BaseURL != "" {
		for i, part := range parts {
			if part.LocalPath != "" && part.URL == "" {
				client := NewActiveStorageClient(c.config.ActiveStorage)
				contentType := part.ContentType
				if contentType == "" {
					contentType = detectContentType(part.LocalPath, part.Type)
				}
				filename := part.Filename
				if filename == "" {
					filename = filepath.Base(part.LocalPath)
				}

				uploadResult, err := client.UploadFile(ctx, part.LocalPath, filename, contentType)
				if err != nil {
					logger.ErrorCF("krabot", "Failed to upload to ActiveStorage", map[string]any{
						"error": err.Error(),
					})
					continue
				}

				signedURL, err := client.GetSignedURL(ctx, uploadResult.SignedID, c.config.ActiveStorage.GetDefaultExpiry())
				if err != nil {
					logger.ErrorCF("krabot", "Failed to get signed URL", map[string]any{
						"error": err.Error(),
					})
					continue
				}

				parts[i].URL = signedURL
				parts[i].LocalPath = "" // Clear after upload
			}
		}
	}

	// Filter out parts that still don't have a URL (if ActiveStorage not configured,
	// we might need to handle this differently - for now, skip them)
	validParts := make([]MediaPart, 0, len(parts))
	for _, part := range parts {
		if part.URL != "" || part.LocalPath != "" {
			// If we have LocalPath but no URL and no ActiveStorage, we can't send it
			// The client needs a URL to download the file
			if part.URL == "" && part.LocalPath != "" {
				logger.WarnCF("krabot", "Skipping media part: no URL and no ActiveStorage configured", map[string]any{
					"filename": part.Filename,
				})
				continue
			}
			validParts = append(validParts, part)
		}
	}

	if len(validParts) == 0 {
		return fmt.Errorf("no valid media parts to send after processing")
	}

	// Use existing method to send to WebSocket clients
	outMsg := newMediaMessage(validParts)
	return c.broadcastToSession(msg.ChatID, outMsg)
}

// broadcastToSession sends a message to all connections in a session.
func (c *KrabotChannel) broadcastToSession(chatID string, msg KrabotMessage) error {
	// chatID format: "krabot:{sessionID}"
	sessionID := strings.TrimPrefix(chatID, "krabot:")
	if sessionID == "" {
		sessionID = chatID
	}
	msg.SessionID = sessionID

	var sent bool
	c.connections.Range(func(key, value any) bool {
		kc, ok := value.(*krabotConn)
		if !ok {
			return true
		}
		if kc.sessionID == sessionID {
			if err := kc.writeJSON(msg); err != nil {
				logger.DebugCF("krabot", "Write to connection failed", map[string]any{
					"conn_id": kc.id,
					"error":   err.Error(),
				})
			} else {
				sent = true
			}
		}
		return true
	})

	if !sent {
		return fmt.Errorf("no active connections for session %s: %w", sessionID, channels.ErrSendFailed)
	}
	return nil
}

// getOrCreateSession gets or creates a session.
func (c *KrabotChannel) getOrCreateSession(sessionID string) *krabotSession {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()

	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	session, ok := c.sessions.Load(sessionID)
	if ok {
		return session.(*krabotSession)
	}

	newSession := &krabotSession{
		id:        sessionID,
		connIDs:   make(map[string]bool),
		createdAt: time.Now().Unix(),
	}
	c.sessions.Store(sessionID, newSession)
	return newSession
}
