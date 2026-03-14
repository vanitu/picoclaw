package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// processTask processes a task through the AI via MessageBus.
func (c *A2AChannel) processTask(task *Task) {
	logger.InfoCF("a2a", "Processing task", map[string]any{
		"task_id": task.ID,
	})

	// Update status to working and notify subscribers
	task.UpdateStatus(TaskStateWorking, nil)
	c.notifySubscribers(task.ID, &StreamResponse{
		StatusUpdate: &TaskStatusUpdateEvent{
			ID:     task.ID,
			Status: task.Status,
		},
	})

	// Get the last user message
	if len(task.History) == 0 {
		task.UpdateStatus(TaskStateFailed, &Message{
			Role: "agent",
			Parts: []Part{{
				Type: PartTypeText,
				Text: "No messages in task history",
			}},
		})
		return
	}

	lastMsg := task.History[len(task.History)-1]

	// Convert A2A message to inbound message
	converter := NewConverter(c)
	inbound, err := converter.PartsToInbound(task.ID, task.SessionID, lastMsg.Parts)
	if err != nil {
		logger.ErrorCF("a2a", "Failed to convert message", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		task.UpdateStatus(TaskStateFailed, &Message{
			Role: "agent",
			Parts: []Part{{
				Type: PartTypeText,
				Text: fmt.Sprintf("Message conversion failed: %v", err),
			}},
		})
		return
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.config.GetTaskTimeout())*time.Minute)
	defer cancel()

	// Start response handler in background
	go c.handleTaskResponse(ctx, task, converter)

	// Publish to MessageBus
	if err := c.bus.PublishInbound(ctx, *inbound); err != nil {
		logger.ErrorCF("a2a", "Failed to publish message", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		task.UpdateStatus(TaskStateFailed, &Message{
			Role: "agent",
			Parts: []Part{{
				Type: PartTypeText,
				Text: fmt.Sprintf("Failed to send message: %v", err),
			}},
		})
	}
}

// handleTaskResponse handles the AI response for a task.
func (c *A2AChannel) handleTaskResponse(ctx context.Context, task *Task, converter *Converter) {
	// Subscribe to outbound messages
	for {
		select {
		case <-ctx.Done():
			// Timeout or cancellation
			if ctx.Err() == context.DeadlineExceeded {
				task.UpdateStatus(TaskStateFailed, &Message{
					Role: "agent",
					Parts: []Part{{
						Type: PartTypeText,
						Text: "Task timeout",
					}},
				})
				c.notifySubscribers(task.ID, &StreamResponse{
					StatusUpdate: &TaskStatusUpdateEvent{
						ID:     task.ID,
						Status: task.Status,
						Final:  true,
					},
				})
			}
			return

		default:
			// Try to get outbound message
			outbound, ok := c.bus.SubscribeOutbound(ctx)
			if !ok {
				continue
			}

			// Check if this message is for our task
			expectedChatID := fmt.Sprintf("a2a:%s", task.ID)
			if outbound.ChatID != expectedChatID {
				continue
			}

			// Build artifact parts
			var parts []Part

			// Add text content
			if outbound.Content != "" {
				parts = append(parts, Part{
					Type: PartTypeText,
					Text: outbound.Content,
				})
			}

			// Handle text response
			if outbound.Content != "" {
				parts = append(parts, Part{
					Type: PartTypeText,
					Text: outbound.Content,
				})
			}

			// Add artifact with text parts
			if len(parts) > 0 {
				task.AddArtifact("response", parts)
			}

			// Check for media attachments (files)
			if c.config.ActiveStorage.IsConfigured() {
				mediaMsg, ok := c.bus.SubscribeOutboundMedia(ctx)
				if ok && mediaMsg.ChatID == expectedChatID && len(mediaMsg.Parts) > 0 {
					// Convert media parts to file parts and upload
					mediaParts := c.convertMediaParts(mediaMsg.Parts, converter)
					if len(mediaParts) > 0 {
						task.AddArtifact("media", mediaParts)
					}
				}
			}

			// Mark task as completed
			task.UpdateStatus(TaskStateCompleted, nil)

			// Notify subscribers
			c.notifySubscribers(task.ID, &StreamResponse{
				Task: task,
				StatusUpdate: &TaskStatusUpdateEvent{
					ID:     task.ID,
					Status: task.Status,
					Final:  true,
				},
			})

			return
		}
	}
}
