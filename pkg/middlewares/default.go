// Package middlewares provides default implementations for agent callbacks.
package middlewares

import (
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// DefaultMiddleware provides sensible default behavior for agent callbacks.
// It tracks tool execution and handles empty LLM responses by forcing
// an additional iteration with a system prompt.
type DefaultMiddleware struct {
	ToolsAttempted int
	ToolsSucceeded int
	ToolsFailed    int
	ToolNames      []string
}

// NewDefault creates a new DefaultMiddleware instance.
func NewDefault() *DefaultMiddleware {
	return &DefaultMiddleware{
		ToolNames: []string{},
	}
}

// OnToolCall is called before a tool is executed.
func (m *DefaultMiddleware) OnToolCall(name string, args map[string]any) {
	m.ToolsAttempted++
	m.ToolNames = append(m.ToolNames, name)
}

// OnToolResult is called after a tool result is received.
func (m *DefaultMiddleware) OnToolResult(name string, result *tools.ToolResult) {
	if result.IsError || result.Err != nil {
		m.ToolsFailed++
	} else {
		m.ToolsSucceeded++
	}
}

// OnEmptyFinalAnswer is called when the LLM returns empty content.
// Returns a fallback message when tools were executed but no response was given.
func (m *DefaultMiddleware) OnEmptyFinalAnswer(iteration int, maxIterations int) string {
	if m.ToolsAttempted > 0 {
		return "I checked the requested information but have nothing to report."
	}
	return ""
}

// OnEmptyFinalAnswerExtended is called when the LLM returns empty content after tools.
// It requests one more iteration with a system prompt to encourage a user-facing response.
func (m *DefaultMiddleware) OnEmptyFinalAnswerExtended(
	messages []providers.Message,
	iteration int,
	maxIterations int,
) ([]providers.Message, bool) {
	// Don't force iteration if at the limit
	if iteration >= maxIterations {
		return messages, false
	}

	// Add system message prompting for user-facing response
	modified := append(messages, providers.Message{
		Role: "system",
		Content: "You executed tools but returned empty content. " +
			"Please provide a user-facing response summarizing what you found.",
	})

	return modified, true
}
