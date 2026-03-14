package a2a

import (
	"encoding/json"
	"time"
)

// A2A Protocol Types based on Google A2A Specification v1.0
// https://a2a-protocol.org

// Task states
const (
	TaskStateSubmitted      = "submitted"
	TaskStateWorking        = "working"
	TaskStateInputRequired  = "input-required"
	TaskStateCompleted      = "completed"
	TaskStateFailed         = "failed"
	TaskStateCanceled       = "canceled"
	TaskStateRejected       = "rejected"
)

// JSON-RPC methods
const (
	MethodSendMessage      = "sendMessage"
	MethodGetTask          = "tasks/get"
	MethodCancelTask       = "tasks/cancel"
	MethodSubscribeTask    = "tasks/subscribe"
	MethodGetAgentCard     = "agent/get"
)

// Part types
const (
	PartTypeText = "text"
	PartTypeFile = "file"
	PartTypeData = "data"
)

// JSON-RPC Request/Response

// JSONRPCRequest is a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// JSONRPCError is a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    interface{}     `json:"data,omitempty"`
}

// A2A Core Types

// Task represents a unit of work
type Task struct {
	ID        string      `json:"id"`
	SessionID string      `json:"sessionId,omitempty"`
	Status    TaskStatus  `json:"status"`
	History   []Message   `json:"history,omitempty"`
	Artifacts []Artifact  `json:"artifacts,omitempty"`
	Metadata  Metadata    `json:"metadata,omitempty"`
}

// TaskStatus represents the current state of a task
type TaskStatus struct {
	State     string     `json:"state"`
	Message   *Message   `json:"message,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// Message represents a communication turn
type Message struct {
	Role      string    `json:"role"`
	Parts     []Part    `json:"parts"`
	Metadata  Metadata  `json:"metadata,omitempty"`
}

// Part represents content (text, file, or data)
type Part struct {
	Type string `json:"type"`
	
	// For text parts
	Text string `json:"text,omitempty"`
	
	// For file parts
	File *FilePart `json:"file,omitempty"`
	
	// For data parts
	Data interface{} `json:"data,omitempty"`
}

// FilePart represents a file reference
type FilePart struct {
	Name     string `json:"name"`
	MimeType string `json:"mimeType,omitempty"`
	Bytes    string `json:"bytes,omitempty"`    // base64 encoded (fallback)
	URI      string `json:"uri,omitempty"`      // URL to file (preferred)
}

// Artifact represents task output
type Artifact struct {
	Name      string     `json:"name"`
	Parts     []Part     `json:"parts"`
	Metadata  Metadata   `json:"metadata,omitempty"`
}

// Metadata is a flexible key-value map
type Metadata map[string]interface{}

// Agent Card Types

// AgentCard describes an agent's capabilities
type AgentCard struct {
	Name               string              `json:"name"`
	Description        string              `json:"description,omitempty"`
	Version            string              `json:"version"`
	Capabilities       AgentCapabilities   `json:"capabilities"`
	DefaultInputModes  []string            `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string            `json:"defaultOutputModes,omitempty"`
	Skills             []AgentSkill        `json:"skills,omitempty"`
}

// AgentCapabilities describes optional features
type AgentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// AgentSkill describes a specific capability
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// Request/Response Types

// SendMessageRequest is the params for sendMessage
type SendMessageRequest struct {
	Message  Message       `json:"message"`
	TaskID   string        `json:"taskId,omitempty"`
	SessionID string       `json:"sessionId,omitempty"`
}

// SendMessageResponse is the result of sendMessage
type SendMessageResponse struct {
	Task *Task `json:"task,omitempty"`
}

// GetTaskRequest is the params for tasks/get
type GetTaskRequest struct {
	TaskID        string `json:"taskId"`
	HistoryLength *int   `json:"historyLength,omitempty"`
}

// CancelTaskRequest is the params for tasks/cancel
type CancelTaskRequest struct {
	TaskID string `json:"taskId"`
}

// SubscribeTaskRequest is the params for tasks/subscribe
type SubscribeTaskRequest struct {
	TaskID string `json:"taskId"`
}

// TaskUpdateEvent is sent during streaming
type TaskUpdateEvent struct {
	Task *Task `json:"task"`
}

// TaskStatusUpdateEvent represents a status change
type TaskStatusUpdateEvent struct {
	ID        string     `json:"id"`
	Status    TaskStatus `json:"status"`
	Final     bool       `json:"final,omitempty"`
}

// TaskArtifactUpdateEvent represents new artifacts
type TaskArtifactUpdateEvent struct {
	ID        string     `json:"id"`
	Artifact  Artifact   `json:"artifact"`
}

// StreamResponse wraps streaming events
type StreamResponse struct {
	Task           *Task                      `json:"task,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent     `json:"statusUpdate,omitempty"`
	ArtifactUpdate *TaskArtifactUpdateEvent   `json:"artifactUpdate,omitempty"`
}

// Error codes
const (
	ErrCodeParseError          = -32700
	ErrCodeInvalidRequest      = -32600
	ErrCodeMethodNotFound      = -32601
	ErrCodeInvalidParams       = -32602
	ErrCodeInternalError       = -32603
	
	ErrCodeTaskNotFound        = -32001
	ErrCodeTaskNotCancelable   = -32002
	ErrCodeContentTypeNotSupported = -32003
	ErrCodeUnsupportedOperation    = -32004
)

// NewSuccessResponse creates a successful JSON-RPC response
func NewSuccessResponse(id interface{}, result interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

// NewErrorResponse creates an error JSON-RPC response
func NewErrorResponse(id interface{}, code int, message string, data interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
}

// NewTask creates a new task with initial state
func NewTask(id, sessionID string) *Task {
	return &Task{
		ID:        id,
		SessionID: sessionID,
		Status: TaskStatus{
			State:     TaskStateSubmitted,
			Timestamp: time.Now(),
		},
		History:   []Message{},
		Artifacts: []Artifact{},
		Metadata:  make(Metadata),
	}
}

// AddMessage adds a message to task history
func (t *Task) AddMessage(role string, parts []Part) {
	t.History = append(t.History, Message{
		Role:     role,
		Parts:    parts,
		Metadata: make(Metadata),
	})
}

// UpdateStatus updates the task status
func (t *Task) UpdateStatus(state string, message *Message) {
	t.Status = TaskStatus{
		State:     state,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// AddArtifact adds an artifact to the task
func (t *Task) AddArtifact(name string, parts []Part) {
	t.Artifacts = append(t.Artifacts, Artifact{
		Name:     name,
		Parts:    parts,
		Metadata: make(Metadata),
	})
}

// IsTerminal returns true if task is in a terminal state
func (t *Task) IsTerminal() bool {
	switch t.Status.State {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	default:
		return false
	}
}
