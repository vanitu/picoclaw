package a2a

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSuccessResponse(t *testing.T) {
	result := map[string]string{"message": "success"}
	resp := NewSuccessResponse("req-123", result)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-123", resp.ID)
	assert.Equal(t, result, resp.Result)
	assert.Nil(t, resp.Error)
}

func TestNewSuccessResponse_NumericID(t *testing.T) {
	result := Task{ID: "task-1"}
	resp := NewSuccessResponse(42, result)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 42, resp.ID)
	assert.NotNil(t, resp.Result)
}

func TestNewErrorResponse(t *testing.T) {
	data := map[string]string{"field": "username"}
	resp := NewErrorResponse("req-456", -32600, "Invalid Request", data)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-456", resp.ID)
	assert.Nil(t, resp.Result)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32600, resp.Error.Code)
	assert.Equal(t, "Invalid Request", resp.Error.Message)
	assert.Equal(t, data, resp.Error.Data)
}

func TestNewErrorResponse_NoData(t *testing.T) {
	resp := NewErrorResponse(1, -32601, "Method not found", nil)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Nil(t, resp.Error.Data)
}

func TestNewTask(t *testing.T) {
	task := NewTask("task-123", "session-abc")

	assert.Equal(t, "task-123", task.ID)
	assert.Equal(t, "session-abc", task.SessionID)
	assert.Equal(t, TaskStateSubmitted, task.Status.State)
	assert.NotZero(t, task.Status.Timestamp)
	assert.Empty(t, task.History)
	assert.Empty(t, task.Artifacts)
	assert.NotNil(t, task.Metadata)
}

func TestNewTask_EmptySession(t *testing.T) {
	task := NewTask("task-456", "")

	assert.Equal(t, "task-456", task.ID)
	assert.Empty(t, task.SessionID)
	assert.Equal(t, TaskStateSubmitted, task.Status.State)
}

func TestTask_AddMessage(t *testing.T) {
	task := NewTask("task-1", "session-1")

	parts := []Part{
		{Type: PartTypeText, Text: "Hello"},
	}
	task.AddMessage("user", parts)

	assert.Len(t, task.History, 1)
	assert.Equal(t, "user", task.History[0].Role)
	assert.Len(t, task.History[0].Parts, 1)
	assert.Equal(t, "Hello", task.History[0].Parts[0].Text)

	// Add another message
	task.AddMessage("agent", []Part{{Type: PartTypeText, Text: "Hi there!"}})
	assert.Len(t, task.History, 2)
	assert.Equal(t, "agent", task.History[1].Role)
}

func TestTask_UpdateStatus(t *testing.T) {
	task := NewTask("task-1", "session-1")
	assert.Equal(t, TaskStateSubmitted, task.Status.State)

	// Update to working
	task.UpdateStatus(TaskStateWorking, nil)
	assert.Equal(t, TaskStateWorking, task.Status.State)
	assert.Nil(t, task.Status.Message)

	// Update to completed with message
	msg := &Message{
		Role:  "agent",
		Parts: []Part{{Type: PartTypeText, Text: "Done!"}},
	}
	task.UpdateStatus(TaskStateCompleted, msg)
	assert.Equal(t, TaskStateCompleted, task.Status.State)
	assert.NotNil(t, task.Status.Message)
	assert.Equal(t, "Done!", task.Status.Message.Parts[0].Text)
}

func TestTask_AddArtifact(t *testing.T) {
	task := NewTask("task-1", "session-1")
	assert.Empty(t, task.Artifacts)

	parts := []Part{
		{Type: PartTypeText, Text: "Result data"},
	}
	task.AddArtifact("output", parts)

	assert.Len(t, task.Artifacts, 1)
	assert.Equal(t, "output", task.Artifacts[0].Name)
	assert.Len(t, task.Artifacts[0].Parts, 1)

	// Add another artifact
	task.AddArtifact("logs", []Part{{Type: PartTypeFile, File: &FilePart{Name: "log.txt"}}})
	assert.Len(t, task.Artifacts, 2)
	assert.Equal(t, "logs", task.Artifacts[1].Name)
}

func TestTask_IsTerminal(t *testing.T) {
	tests := []struct {
		state   string
		want    bool
	}{
		{TaskStateSubmitted, false},
		{TaskStateWorking, false},
		{TaskStateInputRequired, false},
		{TaskStateCompleted, true},
		{TaskStateFailed, true},
		{TaskStateCanceled, true},
		{TaskStateRejected, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			task := NewTask("task-1", "session-1")
			task.Status.State = tt.state
			got := task.IsTerminal()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPart_Structures(t *testing.T) {
	// Text part
	textPart := Part{
		Type: PartTypeText,
		Text: "Hello world",
	}
	assert.Equal(t, PartTypeText, textPart.Type)
	assert.Equal(t, "Hello world", textPart.Text)
	assert.Nil(t, textPart.File)
	assert.Nil(t, textPart.Data)

	// File part
	filePart := Part{
		Type: PartTypeFile,
		File: &FilePart{
			Name:     "document.pdf",
			MimeType: "application/pdf",
			Bytes:    "base64encodeddata",
		},
	}
	assert.Equal(t, PartTypeFile, filePart.Type)
	assert.NotNil(t, filePart.File)
	assert.Equal(t, "document.pdf", filePart.File.Name)

	// Data part
	dataPart := Part{
		Type: PartTypeData,
		Data: map[string]interface{}{
			"key":    "value",
			"number": 42,
		},
	}
	assert.Equal(t, PartTypeData, dataPart.Type)
	assert.NotNil(t, dataPart.Data)
}

func TestConstants(t *testing.T) {
	// Task states
	assert.Equal(t, "submitted", TaskStateSubmitted)
	assert.Equal(t, "working", TaskStateWorking)
	assert.Equal(t, "input-required", TaskStateInputRequired)
	assert.Equal(t, "completed", TaskStateCompleted)
	assert.Equal(t, "failed", TaskStateFailed)
	assert.Equal(t, "canceled", TaskStateCanceled)
	assert.Equal(t, "rejected", TaskStateRejected)

	// JSON-RPC methods
	assert.Equal(t, "sendMessage", MethodSendMessage)
	assert.Equal(t, "tasks/get", MethodGetTask)
	assert.Equal(t, "tasks/cancel", MethodCancelTask)
	assert.Equal(t, "tasks/subscribe", MethodSubscribeTask)
	assert.Equal(t, "agent/get", MethodGetAgentCard)

	// Part types
	assert.Equal(t, "text", PartTypeText)
	assert.Equal(t, "file", PartTypeFile)
	assert.Equal(t, "data", PartTypeData)

	// Error codes
	assert.Equal(t, -32700, ErrCodeParseError)
	assert.Equal(t, -32600, ErrCodeInvalidRequest)
	assert.Equal(t, -32601, ErrCodeMethodNotFound)
	assert.Equal(t, -32602, ErrCodeInvalidParams)
	assert.Equal(t, -32603, ErrCodeInternalError)
	assert.Equal(t, -32001, ErrCodeTaskNotFound)
	assert.Equal(t, -32002, ErrCodeTaskNotCancelable)
	assert.Equal(t, -32003, ErrCodeContentTypeNotSupported)
	assert.Equal(t, -32004, ErrCodeUnsupportedOperation)
}
