package a2a

import (
	"encoding/json"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// handleSendMessage processes sendMessage JSON-RPC method
func (c *A2AChannel) handleSendMessage(req JSONRPCRequest) JSONRPCResponse {
	var params SendMessageRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", nil)
	}

	// Use existing task or create new one
	var task *Task
	var err error

	if params.TaskID != "" {
		// Continue existing task
		task, _ = c.GetTask(params.TaskID)
		if task == nil {
			return NewErrorResponse(req.ID, ErrCodeTaskNotFound, "Task not found", nil)
		}
		if task.IsTerminal() {
			return NewErrorResponse(req.ID, ErrCodeUnsupportedOperation, "Task is terminal", nil)
		}
		task.AddMessage("user", params.Message.Parts)
	} else {
		// Create new task
		task, err = c.CreateTask(params.SessionID, params.Message)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternalError, err.Error(), nil)
		}
	}

	// Update status to working
	task.UpdateStatus(TaskStateWorking, nil)

	// Process through AI (async)
	go c.processTask(task)

	return NewSuccessResponse(req.ID, SendMessageResponse{Task: task})
}

// handleGetTask processes tasks/get JSON-RPC method
func (c *A2AChannel) handleGetTask(req JSONRPCRequest) JSONRPCResponse {
	var params GetTaskRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", nil)
	}

	task, ok := c.GetTask(params.TaskID)
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeTaskNotFound, "Task not found", nil)
	}

	// Apply history length limit if specified
	if params.HistoryLength != nil && *params.HistoryLength >= 0 {
		task = c.limitTaskHistory(task, *params.HistoryLength)
	}

	return NewSuccessResponse(req.ID, task)
}

// handleCancelTask processes tasks/cancel JSON-RPC method
func (c *A2AChannel) handleCancelTask(req JSONRPCRequest) JSONRPCResponse {
	var params CancelTaskRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", nil)
	}

	task, ok := c.GetTask(params.TaskID)
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeTaskNotFound, "Task not found", nil)
	}

	if task.IsTerminal() {
		return NewErrorResponse(req.ID, ErrCodeTaskNotCancelable, "Task not cancelable", nil)
	}

	task.UpdateStatus(TaskStateCanceled, nil)

	return NewSuccessResponse(req.ID, task)
}

// handleGetAgentCard processes agent/get JSON-RPC method
func (c *A2AChannel) handleGetAgentCard(req JSONRPCRequest) JSONRPCResponse {
	card := c.config.BuildAgentCard()
	return NewSuccessResponse(req.ID, card)
}

// handleGetTaskREST handles GET /a2a/tasks/:id
func (c *A2AChannel) handleGetTaskREST(w http.ResponseWriter, r *http.Request, taskID string) {
	task, ok := c.GetTask(taskID)
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleCancelTaskREST handles POST /a2a/tasks/:id/cancel
func (c *A2AChannel) handleCancelTaskREST(w http.ResponseWriter, r *http.Request, taskID string) {
	task, ok := c.GetTask(taskID)
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.IsTerminal() {
		http.Error(w, "Task not cancelable", http.StatusBadRequest)
		return
	}

	task.UpdateStatus(TaskStateCanceled, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// limitTaskHistory limits the task history to the specified length
func (c *A2AChannel) limitTaskHistory(task *Task, limit int) *Task {
	if limit == 0 {
		task.History = nil
		return task
	}

	if len(task.History) > limit {
		task.History = task.History[len(task.History)-limit:]
	}

	return task
}

// processTask processes a task through the AI
func (c *A2AChannel) processTask(task *Task) {
	logger.InfoCF("a2a", "Processing task", map[string]any{
		"task_id": task.ID,
	})

	// This is where the task would be processed through the AI
	// For now, this is a placeholder that would be integrated with:
	// - The agent loop
	// - The message bus
	// - The LLM provider

	// Example flow:
	// 1. Convert task history to format for LLM
	// 2. Send to LLM
	// 3. Stream updates via notifySubscribers
	// 4. Create artifacts for outputs
	// 5. Update status to completed/failed

	// Placeholder: simulate completion
	// In real implementation, this would be async processing
}
