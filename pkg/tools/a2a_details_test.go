package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/a2a"
	"github.com/sipeed/picoclaw/pkg/config"
)

// TestA2ADetailsTool_Name tests the tool name
func TestA2ADetailsTool_Name(t *testing.T) {
	tool := NewA2ADetailsTool(&MockA2ARegistryClient{})
	if tool.Name() != "agent_details" {
		t.Errorf("Expected name 'agent_details', got '%s'", tool.Name())
	}
}

// TestA2ADetailsTool_Description tests the tool description
func TestA2ADetailsTool_Description(t *testing.T) {
	tool := NewA2ADetailsTool(&MockA2ARegistryClient{})
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(tool.Description(), "detailed information") {
		t.Error("Description should mention detailed information")
	}
}

// TestA2ADetailsTool_Parameters tests the parameter schema
func TestA2ADetailsTool_Parameters(t *testing.T) {
	tool := NewA2ADetailsTool(&MockA2ARegistryClient{})
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

	if _, exists := properties["agent_name"]; !exists {
		t.Error("Missing property: agent_name")
	}

	// Check required fields
	reqFields, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Required should be a string slice")
	}
	if len(reqFields) != 1 || reqFields[0] != "agent_name" {
		t.Errorf("Expected required fields [agent_name], got %v", reqFields)
	}
}

// TestA2ADetailsTool_Execute_MissingAgentName tests validation
func TestA2ADetailsTool_Execute_MissingAgentName(t *testing.T) {
	tool := NewA2ADetailsTool(&MockA2ARegistryClient{})
	ctx := context.Background()

	// Missing agent_name
	result := tool.Execute(ctx, map[string]any{})
	if !result.IsError {
		t.Error("Expected error for missing agent_name")
	}
	if result.Err == nil || result.Err.Error() != "agent_name missing" {
		t.Error("Expected specific error for missing agent_name")
	}

	// Empty agent_name
	result = tool.Execute(ctx, map[string]any{"agent_name": ""})
	if !result.IsError {
		t.Error("Expected error for empty agent_name")
	}
}

// TestA2ADetailsTool_Execute_AgentNotFound tests agent lookup failure
func TestA2ADetailsTool_Execute_AgentNotFound(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return nil, false
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "unknown-agent",
	})

	if !result.IsError {
		t.Error("Expected error for unknown agent")
	}
	if result.Err == nil || result.Err.Error() != "agent not found" {
		t.Error("Expected specific error for agent not found")
	}
}

// TestA2ADetailsTool_Execute_AgentUnhealthy tests unhealthy agent
func TestA2ADetailsTool_Execute_AgentUnhealthy(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config:    config.A2ARegistryAgentConfig{Name: "unhealthy-agent"},
				Status:    a2a.StatusUnhealthy,
				Card:      &a2a.AgentCard{Name: "Unhealthy Agent"},
				LastError: "Connection timeout",
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "unhealthy-agent",
	})

	if !result.IsError {
		t.Error("Expected error for unhealthy agent")
	}
	if !strings.Contains(result.ForLLM, "unavailable") {
		t.Errorf("Expected 'unavailable' in error, got: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "Connection timeout") {
		t.Errorf("Expected last error in message, got: %s", result.ForLLM)
	}
}

// TestA2ADetailsTool_Execute_SuccessMinimal tests minimal successful response
func TestA2ADetailsTool_Execute_SuccessMinimal(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "test-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:    "Test Agent",
					Version: "1.0.0",
					Capabilities: a2a.AgentCapabilities{
						Streaming:              true,
						PushNotifications:      false,
						StateTransitionHistory: true,
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "test-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Check content
	if !strings.Contains(result.ForLLM, "Test Agent") {
		t.Error("Expected agent name in result")
	}
	if !strings.Contains(result.ForLLM, "1.0.0") {
		t.Error("Expected version in result")
	}
	if !strings.Contains(result.ForLLM, "Streaming") {
		t.Error("Expected capabilities in result")
	}
}

// TestA2ADetailsTool_Execute_SuccessFull tests full agent card details
func TestA2ADetailsTool_Execute_SuccessFull(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "full-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:        "Full Agent",
					Description: "An agent with full capabilities",
					Version:     "2.1.0",
					Capabilities: a2a.AgentCapabilities{
						Streaming:              true,
						PushNotifications:      true,
						StateTransitionHistory: false,
					},
					DefaultInputModes:  []string{"text", "file"},
					DefaultOutputModes: []string{"text", "image"},
					Skills: []a2a.AgentSkill{
						{
							ID:          "translate",
							Name:        "Translation",
							Description: "Translates text between languages",
							Tags:        []string{"nlp", "language"},
							Examples:    []string{"Translate 'hello' to French", "Translate document to Spanish"},
						},
						{
							ID:          "summarize",
							Name:        "Summarization",
							Description: "Summarizes long documents",
							Tags:        []string{"nlp"},
						},
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "full-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Check all content is present
	checks := []string{
		"Full Agent",
		"An agent with full capabilities",
		"2.1.0",
		"Streaming",
		"Push Notifications",
		"State Transition History",
		"text",
		"file",
		"image",
		"Translation",
		"translate",
		"Translates text between languages",
		"nlp",
		"language",
		"Translate 'hello' to French",
		"Summarization",
		"summarize",
		"call_remote_agent",
	}

	for _, check := range checks {
		if !strings.Contains(result.ForLLM, check) {
			t.Errorf("Expected result to contain '%s'", check)
		}
	}
}

// TestA2ADetailsTool_Execute_NoDescription tests agent without description
func TestA2ADetailsTool_Execute_NoDescription(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "no-desc-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:    "No Description Agent",
					Version: "1.0.0",
					Capabilities: a2a.AgentCapabilities{
						Streaming: false,
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "no-desc-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "No Description Agent") {
		t.Error("Expected agent name in result")
	}
}

// TestA2ADetailsTool_Execute_NoVersion tests agent without version
func TestA2ADetailsTool_Execute_NoVersion(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "no-ver-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name: "No Version Agent",
					Capabilities: a2a.AgentCapabilities{
						Streaming: true,
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "no-ver-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Should not contain Version section if empty
	if strings.Contains(result.ForLLM, "**Version**") {
		t.Error("Should not show version if empty")
	}
}

// TestA2ADetailsTool_Execute_NoSkills tests agent without skills
func TestA2ADetailsTool_Execute_NoSkills(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "no-skills-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:         "No Skills Agent",
					Version:      "1.0.0",
					Capabilities: a2a.AgentCapabilities{},
					Skills:       []a2a.AgentSkill{},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "no-skills-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Should not contain Skills section if empty
	if strings.Contains(result.ForLLM, "## Skills") {
		t.Error("Should not show skills section if empty")
	}
}

// TestA2ADetailsTool_Execute_NoInputOutputModes tests agent without modes specified
func TestA2ADetailsTool_Execute_NoInputOutputModes(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "no-modes-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:               "No Modes Agent",
					Version:            "1.0.0",
					Capabilities:       a2a.AgentCapabilities{},
					DefaultInputModes:  nil,
					DefaultOutputModes: nil,
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "no-modes-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Should not contain modes if nil
	if strings.Contains(result.ForLLM, "Input Modes") {
		t.Error("Should not show input modes if nil")
	}
	if strings.Contains(result.ForLLM, "Output Modes") {
		t.Error("Should not show output modes if nil")
	}
}

// TestA2ADetailsTool_Execute_SkillWithNoExamples tests skill without examples
func TestA2ADetailsTool_Execute_SkillWithNoExamples(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "test-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:    "Test Agent",
					Version: "1.0.0",
					Capabilities: a2a.AgentCapabilities{
						Streaming: true,
					},
					Skills: []a2a.AgentSkill{
						{
							ID:          "simple-skill",
							Name:        "Simple Skill",
							Description: "A simple skill",
							Tags:        []string{"simple"},
							// No examples
						},
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "test-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "Simple Skill") {
		t.Error("Expected skill name in result")
	}

	// Should not contain Examples section for this skill
	// Note: The markdown structure makes this hard to test precisely,
	// but we verify the skill is present
}

// TestA2ADetailsTool_Execute_SkillWithNoTags tests skill without tags
func TestA2ADetailsTool_Execute_SkillWithNoTags(t *testing.T) {
	registry := &MockA2ARegistryClient{
		getFunc: func(name string) (*a2a.AgentEntry, bool) {
			return &a2a.AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "test-agent"},
				Status: a2a.StatusHealthy,
				Card: &a2a.AgentCard{
					Name:    "Test Agent",
					Version: "1.0.0",
					Capabilities: a2a.AgentCapabilities{
						Streaming: true,
					},
					Skills: []a2a.AgentSkill{
						{
							ID:          "no-tags-skill",
							Name:        "No Tags Skill",
							Description: "A skill without tags",
							// No tags
						},
					},
				},
			}, true
		},
	}

	tool := NewA2ADetailsTool(registry)
	ctx := context.Background()

	result := tool.Execute(ctx, map[string]any{
		"agent_name": "test-agent",
	})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "No Tags Skill") {
		t.Error("Expected skill name in result")
	}
}

// TestA2ADetailsTool_InterfaceCompliance verifies interface implementation
func TestA2ADetailsTool_InterfaceCompliance(t *testing.T) {
	tool := NewA2ADetailsTool(&MockA2ARegistryClient{})
	var _ Tool = tool
}
