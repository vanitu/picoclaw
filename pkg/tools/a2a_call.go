package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/a2a"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// A2ARegistryClient defines the interface needed from the A2A registry.
type A2ARegistryClient interface {
	Get(name string) (*a2a.AgentEntry, bool)
}

// A2AClient defines the interface for A2A protocol operations.
type A2AClient interface {
	SendTask(endpoint, token string, message a2a.Message) (*a2a.Task, error)
	GetTask(endpoint, token, taskID string) (*a2a.Task, error)
}

// A2ACallTool calls a remote A2A agent to delegate a task.
type A2ACallTool struct {
	registry A2ARegistryClient
	client   A2AClient
}

// NewA2ACallTool creates a new A2A call tool.
func NewA2ACallTool(registry A2ARegistryClient, client A2AClient) *A2ACallTool {
	return &A2ACallTool{
		registry: registry,
		client:   client,
	}
}

// Name returns the tool name.
func (t *A2ACallTool) Name() string {
	return "call_remote_agent"
}

// Description returns the tool description.
func (t *A2ACallTool) Description() string {
	return "Call a remote A2A agent to delegate a task. " +
		"Use agent_details to see full capabilities before calling if unsure."
}

// Parameters returns the tool parameters schema.
func (t *A2ACallTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the remote agent to call (e.g., 'code-reviewer', 'translator')",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task to delegate to the remote agent. Be specific and include all necessary context.",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Additional context or background information for the task (optional)",
			},
			"wait_for_result": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to wait for the task to complete (true) or return immediately with task ID (false). Default: true",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "number",
				"description": "Maximum time to wait for result in seconds. Default: 60",
			},
		},
		"required": []string{"agent_name", "task"},
	}
}

// Execute calls a remote A2A agent.
func (t *A2ACallTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	agentName, ok := args["agent_name"].(string)
	if !ok || agentName == "" {
		return ErrorResult("agent_name is required").WithError(fmt.Errorf("agent_name missing"))
	}

	task, ok := args["task"].(string)
	if !ok || task == "" {
		return ErrorResult("task is required").WithError(fmt.Errorf("task missing"))
	}

	context, _ := args["context"].(string)
	waitForResult := true
	if v, ok := args["wait_for_result"].(bool); ok {
		waitForResult = v
	}

	timeout := 60
	if v, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(v)
	}

	// Get agent from registry
	entry, ok := t.registry.Get(agentName)
	if !ok {
		return ErrorResult(fmt.Sprintf("Agent '%s' not found", agentName)).
			WithError(fmt.Errorf("agent not in registry"))
	}

	// Check if agent is healthy
	if !entry.IsHealthy() {
		logger.WarnCF("a2a", "Call to unhealthy agent rejected", map[string]any{
			"agent_name": agentName,
			"status":     entry.Status.String(),
		})
		return ErrorResult(fmt.Sprintf("Agent '%s' is currently unavailable (status: %s)",
			agentName, entry.Status.String())).
			WithError(fmt.Errorf("agent unhealthy"))
	}

	// Build message
	fullTask := task
	if context != "" {
		fullTask = fmt.Sprintf("%s\n\nContext: %s", task, context)
	}

	message := a2a.Message{
		Role: "user",
		Parts: []a2a.Part{
			{Type: "text", Text: fullTask},
		},
	}

	logger.InfoCF("a2a", "Sending task to remote agent", map[string]any{
		"agent_name": agentName,
		"endpoint":   entry.Config.Endpoint,
	})

	// Send task
	start := time.Now()
	remoteTask, err := t.client.SendTask(entry.Config.Endpoint, entry.Config.Token, message)
	if err != nil {
		logger.ErrorCF("a2a", "Failed to send task", map[string]any{
			"agent_name": agentName,
			"error":      err.Error(),
		})
		return ErrorResult(fmt.Sprintf("Failed to send task to '%s': %v", agentName, err)).
			WithError(err)
	}

	logger.InfoCF("a2a", "Task sent successfully", map[string]any{
		"agent_name": agentName,
		"task_id":    remoteTask.ID,
	})

	// If not waiting, return immediately
	if !waitForResult {
		return &ToolResult{
			ForLLM: fmt.Sprintf("Task sent to agent '%s'. Task ID: %s. "+
				"Use get_task_status or wait for callback.", agentName, remoteTask.ID),
			ForUser: fmt.Sprintf("Task delegated to %s", agentName),
		}
	}

	// Poll for result
	result, err := t.pollForResult(ctx, entry, remoteTask.ID, timeout)
	duration := time.Since(start)

	if err != nil {
		logger.ErrorCF("a2a", "Task failed or timed out", map[string]any{
			"agent_name": agentName,
			"task_id":    remoteTask.ID,
			"duration":   duration.String(),
			"error":      err.Error(),
		})
		return ErrorResult(fmt.Sprintf("Task failed: %v", err)).WithError(err)
	}

	logger.InfoCF("a2a", "Task completed", map[string]any{
		"agent_name": agentName,
		"task_id":    remoteTask.ID,
		"duration":   duration.String(),
	})

	return &ToolResult{
		ForLLM:  fmt.Sprintf("Agent '%s' completed the task:\n\n%s", agentName, result),
		ForUser: fmt.Sprintf("Received response from %s", agentName),
	}
}

// pollForResult polls the remote agent until task completion or timeout.
func (t *A2ACallTool) pollForResult(ctx context.Context, entry *a2a.AgentEntry, taskID string, timeout int) (string, error) {
	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return "", fmt.Errorf("timeout waiting for task completion")
		case <-ticker.C:
			task, err := t.client.GetTask(entry.Config.Endpoint, entry.Config.Token, taskID)
			if err != nil {
				return "", fmt.Errorf("failed to get task status: %w", err)
			}

			if task.IsTerminal() {
				return t.extractResult(task)
			}
		}
	}
}

// extractResult extracts the result text from a completed task.
func (t *A2ACallTool) extractResult(task *a2a.Task) (string, error) {
	if task.Status.State == a2a.TaskStateFailed {
		if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
			return "", fmt.Errorf("task failed: %s", task.Status.Message.Parts[0].Text)
		}
		return "", fmt.Errorf("task failed")
	}

	if task.Status.State == a2a.TaskStateCanceled {
		return "", fmt.Errorf("task was canceled")
	}

	// Extract from artifacts
	var results []string
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if part.Type == "text" && part.Text != "" {
				results = append(results, part.Text)
			}
		}
	}

	if len(results) == 0 {
		return "Task completed with no output", nil
	}

	return results[0], nil
}
