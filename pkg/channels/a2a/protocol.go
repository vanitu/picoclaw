package a2a

// Re-export all protocol types from the shared a2a package
// to maintain backward compatibility with existing code.

import (
	shared "github.com/sipeed/picoclaw/pkg/a2a"
)

// Re-export all types
type (
	JSONRPCRequest               = shared.JSONRPCRequest
	JSONRPCResponse              = shared.JSONRPCResponse
	JSONRPCError                 = shared.JSONRPCError
	Task                         = shared.Task
	TaskStatus                   = shared.TaskStatus
	Message                      = shared.Message
	Part                         = shared.Part
	FilePart                     = shared.FilePart
	Artifact                     = shared.Artifact
	Metadata                     = shared.Metadata
	AgentCard                    = shared.AgentCard
	AgentCapabilities            = shared.AgentCapabilities
	AgentSkill                   = shared.AgentSkill
	SendMessageRequest           = shared.SendMessageRequest
	SendMessageResponse          = shared.SendMessageResponse
	GetTaskRequest               = shared.GetTaskRequest
	CancelTaskRequest            = shared.CancelTaskRequest
	SubscribeTaskRequest         = shared.SubscribeTaskRequest
	TaskUpdateEvent              = shared.TaskUpdateEvent
	TaskStatusUpdateEvent        = shared.TaskStatusUpdateEvent
	TaskArtifactUpdateEvent      = shared.TaskArtifactUpdateEvent
	StreamResponse               = shared.StreamResponse
)

// Re-export constants
const (
	TaskStateSubmitted     = shared.TaskStateSubmitted
	TaskStateWorking       = shared.TaskStateWorking
	TaskStateInputRequired = shared.TaskStateInputRequired
	TaskStateCompleted     = shared.TaskStateCompleted
	TaskStateFailed        = shared.TaskStateFailed
	TaskStateCanceled      = shared.TaskStateCanceled
	TaskStateRejected      = shared.TaskStateRejected

	MethodSendMessage   = shared.MethodSendMessage
	MethodGetTask       = shared.MethodGetTask
	MethodCancelTask    = shared.MethodCancelTask
	MethodSubscribeTask = shared.MethodSubscribeTask
	MethodGetAgentCard  = shared.MethodGetAgentCard

	PartTypeText = shared.PartTypeText
	PartTypeFile = shared.PartTypeFile
	PartTypeData = shared.PartTypeData

	ErrCodeParseError              = shared.ErrCodeParseError
	ErrCodeInvalidRequest          = shared.ErrCodeInvalidRequest
	ErrCodeMethodNotFound          = shared.ErrCodeMethodNotFound
	ErrCodeInvalidParams           = shared.ErrCodeInvalidParams
	ErrCodeInternalError           = shared.ErrCodeInternalError
	ErrCodeTaskNotFound            = shared.ErrCodeTaskNotFound
	ErrCodeTaskNotCancelable       = shared.ErrCodeTaskNotCancelable
	ErrCodeContentTypeNotSupported = shared.ErrCodeContentTypeNotSupported
	ErrCodeUnsupportedOperation    = shared.ErrCodeUnsupportedOperation
)

// Re-export functions
var (
	NewSuccessResponse = shared.NewSuccessResponse
	NewErrorResponse   = shared.NewErrorResponse
	NewTask            = shared.NewTask
)
