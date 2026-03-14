package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Ensure A2AChannel implements Channel interface
var _ channels.Channel = (*A2AChannel)(nil)

// A2AChannel implements the A2A (Agent-to-Agent) protocol
type A2AChannel struct {
	*channels.BaseChannel
	config A2AConfig

	// HTTP server
	mux    *http.ServeMux
	server *http.Server

	// WebSocket
	upgrader websocket.Upgrader

	// Task management
	tasks      map[string]*Task
	tasksMu    sync.RWMutex
	sessions   map[string]*a2aSession
	sessionsMu sync.RWMutex

	// Streaming
	streamSubs map[string]map[string]chan *StreamResponse
	streamMu   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// a2aSession tracks tasks per session
type a2aSession struct {
	id        string
	taskIDs   []string
	clientIDs map[string]bool
	mu        sync.RWMutex
	createdAt time.Time
}

// NewA2AChannel creates a new A2A protocol channel
func NewA2AChannel(cfg A2AConfig, messageBus *bus.MessageBus) (*A2AChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("a2a token is required")
	}

	base := channels.NewBaseChannel("a2a", cfg, messageBus, nil)

	return &A2AChannel{
		BaseChannel: base,
		config:      cfg,
		mux:         http.NewServeMux(),
		tasks:       make(map[string]*Task),
		sessions:    make(map[string]*a2aSession),
		streamSubs:  make(map[string]map[string]chan *StreamResponse),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}, nil
}

// Start implements Channel
func (c *A2AChannel) Start(ctx context.Context) error {
	logger.InfoC("a2a", "Starting A2A channel")
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Setup HTTP handlers
	c.setupHandlers()

	c.SetRunning(true)
	logger.InfoC("a2a", "A2A channel started")
	return nil
}

// Stop implements Channel
func (c *A2AChannel) Stop(ctx context.Context) error {
	logger.InfoC("a2a", "Stopping A2A channel")
	c.SetRunning(false)

	if c.server != nil {
		c.server.Shutdown(ctx)
	}

	if c.cancel != nil {
		c.cancel()
	}

	logger.InfoC("a2a", "A2A channel stopped")
	return nil
}

// WebhookPath implements channels.WebhookHandler
func (c *A2AChannel) WebhookPath() string { return "/a2a/" }

// ServeHTTP implements http.Handler
func (c *A2AChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mux.ServeHTTP(w, r)
}

// setupHandlers registers HTTP handlers
func (c *A2AChannel) setupHandlers() {
	// Agent Card discovery
	c.mux.HandleFunc("/.well-known/agent.json", c.handleAgentCard)
	c.mux.HandleFunc("/a2a/agent-card", c.handleAgentCard)

	// JSON-RPC endpoint
	c.mux.HandleFunc("/a2a", c.handleJSONRPC)

	// Task operations (REST style)
	c.mux.HandleFunc("/a2a/tasks/", c.handleTasks)

	// WebSocket streaming
	c.mux.HandleFunc("/a2a/stream", c.handleWebSocket)
}

// handleAgentCard serves the Agent Card
func (c *A2AChannel) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	card := c.config.BuildAgentCard()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

// handleJSONRPC handles JSON-RPC requests
func (c *A2AChannel) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate
	if !c.authenticate(r) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(NewErrorResponse(nil, -32600, "Unauthorized", nil))
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(NewErrorResponse(nil, ErrCodeParseError, "Parse error", nil))
		return
	}

	// Process method
	var resp JSONRPCResponse
	switch req.Method {
	case MethodSendMessage:
		resp = c.handleSendMessage(req)
	case MethodGetTask:
		resp = c.handleGetTask(req)
	case MethodCancelTask:
		resp = c.handleCancelTask(req)
	case MethodGetAgentCard:
		resp = c.handleGetAgentCard(req)
	default:
		resp = NewErrorResponse(req.ID, ErrCodeMethodNotFound, "Method not found", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleTasks handles REST-style task operations
func (c *A2AChannel) handleTasks(w http.ResponseWriter, r *http.Request) {
	if !c.authenticate(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/a2a/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	taskID := parts[0]

	switch r.Method {
	case http.MethodGet:
		c.handleGetTaskREST(w, r, taskID)
	case http.MethodPost:
		if len(parts) > 1 && parts[1] == "cancel" {
			c.handleCancelTaskREST(w, r, taskID)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// authenticate checks the Bearer token
func (c *A2AChannel) authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	token := ""
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		token = after
	}
	return token == c.config.Token
}

// CreateTask creates a new task
func (c *A2AChannel) CreateTask(sessionID string, message Message) (*Task, error) {
	if !c.IsRunning() {
		return nil, fmt.Errorf("channel not running")
	}

	// Check max tasks
	if len(c.tasks) >= c.config.GetMaxTasks() {
		return nil, fmt.Errorf("max tasks reached")
	}

	taskID := uuid.New().String()
	task := NewTask(taskID, sessionID)
	task.AddMessage("user", message.Parts)

	c.tasksMu.Lock()
	c.tasks[taskID] = task
	c.tasksMu.Unlock()

	// Add to session
	c.addTaskToSession(sessionID, taskID)

	logger.InfoCF("a2a", "Task created", map[string]any{
		"task_id":    taskID,
		"session_id": sessionID,
	})

	return task, nil
}

// GetTask retrieves a task by ID
func (c *A2AChannel) GetTask(taskID string) (*Task, bool) {
	c.tasksMu.RLock()
	defer c.tasksMu.RUnlock()
	task, ok := c.tasks[taskID]
	return task, ok
}

// UpdateTaskStatus updates a task's status
func (c *A2AChannel) UpdateTaskStatus(taskID string, state string, message *Message) error {
	task, ok := c.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task not found")
	}

	task.UpdateStatus(state, message)

	// Notify subscribers
	c.notifySubscribers(taskID, &StreamResponse{
		StatusUpdate: &TaskStatusUpdateEvent{
			ID:     taskID,
			Status: task.Status,
			Final:  task.IsTerminal(),
		},
	})

	return nil
}

// AddTaskArtifact adds an artifact to a task
func (c *A2AChannel) AddTaskArtifact(taskID string, artifact Artifact) error {
	task, ok := c.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task not found")
	}

	task.Artifacts = append(task.Artifacts, artifact)

	// Notify subscribers
	c.notifySubscribers(taskID, &StreamResponse{
		ArtifactUpdate: &TaskArtifactUpdateEvent{
			ID:       taskID,
			Artifact: artifact,
		},
	})

	return nil
}

// addTaskToSession tracks a task in its session
func (c *A2AChannel) addTaskToSession(sessionID, taskID string) {
	if sessionID == "" {
		return
	}

	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		session = &a2aSession{
			id:        sessionID,
			taskIDs:   []string{},
			clientIDs: make(map[string]bool),
			createdAt: time.Now(),
		}
		c.sessions[sessionID] = session
	}

	session.mu.Lock()
	session.taskIDs = append(session.taskIDs, taskID)
	session.mu.Unlock()
}

// subscribeToTask creates a subscription for task updates
func (c *A2AChannel) subscribeToTask(taskID, clientID string) chan *StreamResponse {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.streamSubs[taskID] == nil {
		c.streamSubs[taskID] = make(map[string]chan *StreamResponse)
	}

	ch := make(chan *StreamResponse, 10)
	c.streamSubs[taskID][clientID] = ch

	return ch
}

// unsubscribeFromTask removes a subscription
func (c *A2AChannel) unsubscribeFromTask(taskID, clientID string) {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if subs, ok := c.streamSubs[taskID]; ok {
		if ch, ok := subs[clientID]; ok {
			close(ch)
			delete(subs, clientID)
		}
	}
}

// Send implements Channel - sends a message (not used in A2A, tasks are used instead)
func (c *A2AChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	// A2A uses tasks, not direct messages
	// This method is required for the Channel interface but not used
	return fmt.Errorf("a2a channel uses tasks, not direct messages")
}

// notifySubscribers sends updates to all subscribers
func (c *A2AChannel) notifySubscribers(taskID string, resp *StreamResponse) {
	c.streamMu.RLock()
	subs := c.streamSubs[taskID]
	c.streamMu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- resp:
		default:
			// Channel full, skip
		}
	}
}
