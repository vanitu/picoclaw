package krabot

import (
	"context"
	"fmt"
	"net/http"
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

// SendMedia sends media to clients via signed URLs.
func (c *KrabotChannel) SendMedia(ctx context.Context, chatID string, media []MediaPart) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	outMsg := newMediaMessage(media)
	return c.broadcastToSession(chatID, outMsg)
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
