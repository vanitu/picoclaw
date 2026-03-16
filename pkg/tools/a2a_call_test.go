package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/a2a"
	"github.com/sipeed/picoclaw/pkg/config"
)

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// MockA2ARegistryClient is a mock implementation of A2ARegistryClient
type MockA2ARegistryClient struct {
	getFunc func(name string) (*a2a.AgentEntry, bool)
}

func (m *MockA2ARegistryClient) Get(name string) (*a2a.AgentEntry, bool) {
	if m.getFunc != nil {
		return m.getFunc(name)
	}
	return nil, false
}

// MockA2AClient is a mock implementation of A2AClient
type MockA2AClient struct {
	sendTaskFunc func(endpoint, token string, message a2a.Message) (*a2a.Task, error)
	getTaskFunc  func(endpoint, token, taskID string) (*a2a.Task, error)
}

func (m *MockA2AClient) SendTask(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
	if m.sendTaskFunc != nil {
		return m.sendTaskFunc(endpoint, token, message)
	}
	return nil, errors.New("send task not implemented")
}

func (m *MockA2AClient) GetTask(endpoint, token, taskID string) (*a2a.Task, error) {
	if m.getTaskFunc != nil {
		return m.getTaskFunc(endpoint, token, taskID)
	}
	return nil, errors.New("get task not implemented")
}

// TestA2ACallTool_Name tests the tool name
func TestA2ACallTool_Name(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	if tool.Name() != "call_remote_agent" {
		t.Errorf("Expected name 'call_remote_agent', got '%s'", tool.Name())
	}
}

// TestA2ACallTool_Description tests the tool description
func TestA2ACallTool_Description(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

// TestA2ACallTool_Parameters tests the parameter schema
func TestA2ACallTool_Parameters(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	params := tool.Parameters()

	if params == nil {
		t.Fatal("Parameters should not be nil")
	}

	// Check type
	if params["type"] != "object" {
		t.Errorf("Expected type 'object', got '%v'", params["type"])
	}

	// Check properties exist
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	required := []string{"agent_name", "task", "context", "wait_for_result", "timeout_seconds"}
	for _, prop := range required {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Missing property: %s", prop)
		}
	}

	// Check required fields
	reqFields, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Required should be a string slice")
	}
	if len(reqFields) != 2 || reqFields[0] != "agent_name" || reqFields[1] != "task" {
		t.Errorf("Expected required fields [agent_name, task], got %v", reqFields)
	}
}

// TestA2ACallTool_Execute_MissingAgentName tests validation
func TestA2ACallTool_Execute_MissingAgentName(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	ctx := context.Background()

	// Missing agent_name
	result := tool.Execute(ctx, map[string]any{"task": "do something"})
	if !result.IsError {
		t.Error("Expected error for missing agent_name")
	}
	if result.Err == nil || result.Err.Error() != "agent_name missing" {
		t.Error("Expected specific error for missing agent_name")
	}

	// Empty agent_name
	result = tool.Execute(ctx, map[string]any{"agent_name": "", "task": "do something"})
	if !result.IsError {
		t.Error("Expected error for empty agent_name")
	}
}

// TestA2ACallTool_Execute_MissingTask tests validation
func TestA2ACallTool_Execute_MissingTask(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	ctx := context.Background()

	// Missing task
	result := tool.Execute(ctx, map[string]any{"agent_name": "test-agent"})
	if !result.IsError {
		t.Error("Expected error for missing task")
	}

	// Empty task
	result = tool.Execute(ctx, map[string]any{"agent_name": "test-agent", "task": ""})
	if !result.IsError {
		t.Error("Expected error for empty task")
	}
}

// TestA2ACallTool_Execute_AgentNotFound tests agent lookup failure
func TestA2ACallTool_Execute_AgentNotFound(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return nil, false
		},
	}

	tool := NewA2ACallTool(registry, &MockA2AClient{})
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "unknown-agent",
		"task":       "do something",
	})

	if !result.IsError {
		t.Error("Expected error for unknown agent")
	}
	if result.Err == nil || result.Err.Error() != "agent not in registry" {
		t.Error("Expected specific error for agent not found")
	}
}

// TestA2ACallTool_Execute_AgentUnhealthy tests unhealthy agent rejection
func TestA2ACallTool_Execute_AgentUnhealthy(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "unhealthy-agent"},
				Status: a2a.StatusUnhealthy,
				Card:   &a2a.AgentCard{Name: "Unhealthy Agent"},
			}, true
		},
	}

	tool := NewA2ACallTool(registry, &MockA2AClient{})
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "unhealthy-agent",
		"task":       "do something",
	})

	if !result.IsError {
		t.Error("Expected error for unhealthy agent")
	}
	if result.Err == nil || result.Err.Error() != "agent unhealthy" {
		t.Error("Expected specific error for unhealthy agent")
	}
}

// TestA2ACallTool_Execute_SendTaskError tests task send failure
func TestA2ACallTool_Execute_SendTaskError(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
					Token:    "test-token",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return nil, errors.New("connection refused")
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "healthy-agent",
		"task":       "do something",
	})

	if !result.IsError {
		t.Error("Expected error when send task fails")
	}
}

// TestA2ACallTool_Execute_SuccessNoWait tests successful task send without waiting
func TestA2ACallTool_Execute_SuccessNoWait(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
					Token:    "test-token",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return &a2a.Task{ID: "task-123"}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "do something",
		"wait_for_result": false,
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" || !contains(result.ForLLM, "task-123") {
		t.Errorf("Expected task ID in result, got: %s", result.ForLLM)
	}
}

// TestA2ACallTool_Execute_SuccessWithWait tests successful task with polling
func TestA2ACallTool_Execute_SuccessWithWait(t *testing.T) {
	callCount := 0
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
					Token:    "test-token",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return &a2a.Task{ID: "task-456"}, nil
		},
		getTaskFunc: func(endpoint, token, taskID string) (*a2a.Task, error) {
			callCount++
			if callCount < 2 {
				// Return working state first
				return &a2a.Task{
					ID: taskID,
					Status: a2a.TaskStatus{
						State: a2a.TaskStateWorking,
					},
				}, nil
			}
			// Then return completed
			return &a2a.Task{
				ID: taskID,
				Status: a2a.TaskStatus{
					State: a2a.TaskStateCompleted,
				},
				Artifacts: []a2a.Artifact{
					{
						Name: "result",
						Parts: []a2a.Part{
							{Type: "text", Text: "Task completed successfully"},
						},
					},
				},
			}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "do something",
		"wait_for_result": true,
		"timeout_seconds": 5.0,
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}
	if !contains(result.ForLLM, "completed successfully") {
		t.Errorf("Expected task result in output, got: %s", result.ForLLM)
	}
}

// TestA2ACallTool_Execute_TaskFailed tests failed task handling
func TestA2ACallTool_Execute_TaskFailed(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return &a2a.Task{ID: "task-fail"}, nil
		},
		getTaskFunc: func(endpoint, token, taskID string) (*a2a.Task, error) {
			return &a2a.Task{
				ID: taskID,
				Status: a2a.TaskStatus{
					State: a2a.TaskStateFailed,
					Message: &a2a.Message{
						Role: "agent",
						Parts: []a2a.Part{
							{Type: "text", Text: "Something went wrong"},
						},
					},
				},
			}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "do something",
		"timeout_seconds": 5.0,
	})

	if !result.IsError {
		t.Error("Expected error for failed task")
	}
}

// TestA2ACallTool_Execute_TaskCanceled tests canceled task handling
func TestA2ACallTool_Execute_TaskCanceled(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return &a2a.Task{ID: "task-cancel"}, nil
		},
		getTaskFunc: func(endpoint, token, taskID string) (*a2a.Task, error) {
			return &a2a.Task{
				ID: taskID,
				Status: a2a.TaskStatus{
					State: a2a.TaskStateCanceled,
				},
			}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "do something",
		"timeout_seconds": 5.0,
	})

	if !result.IsError {
		t.Error("Expected error for canceled task")
	}
	if !contains(result.ForLLM, "canceled") {
		t.Errorf("Expected 'canceled' in error message, got: %s", result.ForLLM)
	}
}

// TestA2ACallTool_Execute_Timeout tests timeout handling
func TestA2ACallTool_Execute_Timeout(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			return &a2a.Task{ID: "task-timeout"}, nil
		},
		getTaskFunc: func(endpoint, token, taskID string) (*a2a.Task, error) {
			// Always return working - never completes
			return &a2a.Task{
				ID: taskID,
				Status: a2a.TaskStatus{
					State: a2a.TaskStateWorking,
				},
			}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "do something",
		"timeout_seconds": 1.0, // 1 second timeout
	})

	if !result.IsError {
		t.Error("Expected error for timeout")
	}
	if !contains(result.ForLLM, "timeout") && !contains(result.ForLLM, "failed") {
		t.Errorf("Expected timeout or failed in error message, got: %s", result.ForLLM)
	}
}

// TestA2ACallTool_Execute_WithContext tests task with additional context
func TestA2ACallTool_Execute_WithContext(t *testing.T) {
	var capturedMessage a2a.Message
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					Name:     "healthy-agent",
					Endpoint: "http://test.example.com",
				},
				Status: a2a.StatusHealthy,
				Card:   &a2a.AgentCard{Name: "Healthy Agent"},
			}, true
		},
	}

	client := &MockA2AClient{
		sendTaskFunc: func(endpoint, token string, message a2a.Message) (*a2a.Task, error) {
			capturedMessage = message
			return &a2a.Task{
				ID: "task-ctx",
				Status: a2a.TaskStatus{
					State: a2a.TaskStateWorking,
				},
			}, nil
		},
		getTaskFunc: func(endpoint, token, taskID string) (*a2a.Task, error) {
			return &a2a.Task{
				ID: taskID,
				Status: a2a.TaskStatus{
					State: a2a.TaskStateCompleted,
				},
				Artifacts: []a2a.Artifact{
					{
						Name: "result",
						Parts: []a2a.Part{
							{Type: "text", Text: "Done"},
						},
					},
				},
			}, nil
		},
	}

	tool := NewA2ACallTool(registry, client)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name":      "healthy-agent",
		"task":            "Translate this text",
		"context":         "The text is from a technical document about APIs",
		"timeout_seconds": 5.0,
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify context was included
	if len(capturedMessage.Parts) == 0 || !contains(capturedMessage.Parts[0].Text, "Context:") {
		t.Error("Expected context to be included in message")
	}
}

// TestA2ACallTool_InterfaceCompliance verifies interface implementation
func TestA2ACallTool_InterfaceCompliance(t *testing.T) {
	tool := NewA2ACallTool(&MockA2ARegistryClient{}, &MockA2AClient{})
	var _ Tool = tool
}
