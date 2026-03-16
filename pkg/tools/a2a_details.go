package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// A2ADetailsTool retrieves detailed information about a remote A2A agent.
type A2ADetailsTool struct {
	registry A2ARegistryClient
}

// NewA2ADetailsTool creates a new A2A details tool.
func NewA2ADetailsTool(registry A2ARegistryClient) *A2ADetailsTool {
	return &A2ADetailsTool{
		registry: registry,
	}
}

// Name returns the tool name.
func (t *A2ADetailsTool) Name() string {
	return "agent_details"
}

// Description returns the tool description.
func (t *A2ADetailsTool) Description() string {
	return "Get detailed information about a remote A2A agent including full capabilities, " +
		"skills, and examples. Use this before calling an agent if you need to understand " +
		"its full capabilities beyond the summary in the context."
}

// Parameters returns the tool parameters schema.
func (t *A2ADetailsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the remote agent to get details for",
			},
		},
		"required": []string{"agent_name"},
	}
}

// Execute retrieves agent details.
func (t *A2ADetailsTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	agentName, ok := args["agent_name"].(string)
	if !ok || agentName == "" {
		return ErrorResult("agent_name is required").WithError(fmt.Errorf("agent_name missing"))
	}

	// Get agent from registry
	entry, ok := t.registry.Get(agentName)
	if !ok {
		logger.WarnCF("a2a", "Agent details requested but agent not found", map[string]any{
			"agent_name": agentName,
		})
		return ErrorResult(fmt.Sprintf("Agent '%s' not found in registry", agentName)).
			WithError(fmt.Errorf("agent not found"))
	}

	// Check if agent is healthy
	if !entry.IsHealthy() {
		logger.WarnCF("a2a", "Agent details requested for unhealthy agent", map[string]any{
			"agent_name": agentName,
			"status":     entry.Status.String(),
			"last_error": entry.LastError,
		})
		return ErrorResult(fmt.Sprintf("Agent '%s' is currently unavailable (status: %s, last error: %s)",
			agentName, entry.Status.String(), entry.LastError)).
			WithError(fmt.Errorf("agent unavailable"))
	}

	card := entry.Card

	// Build detailed response
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Agent: %s\n\n", card.Name))

	if card.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", card.Description))
	}

	if card.Version != "" {
		sb.WriteString(fmt.Sprintf("**Version**: %s\n\n", card.Version))
	}

	// Capabilities
	sb.WriteString("## Capabilities\n\n")
	sb.WriteString(fmt.Sprintf("- **Streaming**: %v\n", card.Capabilities.Streaming))
	sb.WriteString(fmt.Sprintf("- **Push Notifications**: %v\n", card.Capabilities.PushNotifications))
	sb.WriteString(fmt.Sprintf("- **State Transition History**: %v\n\n", card.Capabilities.StateTransitionHistory))

	// Input/Output modes
	if len(card.DefaultInputModes) > 0 {
		sb.WriteString(fmt.Sprintf("**Input Modes**: %s\n\n", strings.Join(card.DefaultInputModes, ", ")))
	}
	if len(card.DefaultOutputModes) > 0 {
		sb.WriteString(fmt.Sprintf("**Output Modes**: %s\n\n", strings.Join(card.DefaultOutputModes, ", ")))
	}

	// Skills
	if len(card.Skills) > 0 {
		sb.WriteString("## Skills\n\n")
		for _, skill := range card.Skills {
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", skill.Name, skill.ID))
			if skill.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n", skill.Description))
			}
			if len(skill.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("*Tags: %s*\n", strings.Join(skill.Tags, ", ")))
			}
			if len(skill.Examples) > 0 {
				sb.WriteString("\n**Examples:**\n")
				for _, example := range skill.Examples {
					sb.WriteString(fmt.Sprintf("- %s\n", example))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Usage info
	sb.WriteString("## Usage\n\n")
	sb.WriteString(fmt.Sprintf("To call this agent:\n```\ncall_remote_agent(agent_name=\"%s\", task=\"your task here\")\n```\n", agentName))

	logger.InfoCF("a2a", "Agent details retrieved", map[string]any{
		"agent_name":   agentName,
		"skills_count": len(card.Skills),
	})

	return &ToolResult{
		ForLLM:  sb.String(),
		ForUser: fmt.Sprintf("Retrieved details for agent '%s'", agentName),
	}
}
