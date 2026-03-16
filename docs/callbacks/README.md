# Agent Loop Callbacks

## Overview

The Agent Loop Callbacks system provides a way to customize agent behavior without modifying picoclaw's core logic. This enables middleware patterns for tracking tool execution, handling empty responses, and extending agent functionality.

## Use Cases

- **Track tool calls and results**: Monitor which tools were called and their outcomes
- **Custom empty response handling**: Provide contextual messages when the LLM returns empty content
- **Force extra iteration**: Request one more LLM call with modified messages when empty response received after tools
- **Logging and analytics**: Record tool usage patterns for debugging or metrics
- **Custom fallback logic**: Implement your own response generation when the agent has no response

## Interface

```go
// AgentLoopCallbacks allows customizing agent behavior without modifying core logic
type AgentLoopCallbacks interface {
    // OnToolCall is called before a tool is executed
    OnToolCall(name string, args map[string]any)
    
    // OnToolResult is called after a tool result is received
    OnToolResult(name string, result *tools.ToolResult)
    
    // OnEmptyFinalAnswer is called when the LLM returns empty content
    // Return a non-empty string to override the default response
    OnEmptyFinalAnswer(iteration int, maxIterations int) string
    
    // OnEmptyFinalAnswerExtended is called when the LLM returns empty content
    // after tools were executed. Return modified messages and true to request
    // one more iteration with the modified messages.
    OnEmptyFinalAnswerExtended(
        messages []providers.Message,
        iteration int,
        maxIterations int,
    ) (modifiedMessages []providers.Message, requestAnotherIteration bool)
}
```

## Setting Up Callbacks

```go
import "github.com/sipeed/picoclaw/pkg/agent"

// Create your middleware
middleware := &MyMiddleware{}

// Create agent loop
agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

// Register callbacks
agentLoop.SetCallbacks(middleware)
```

## Example Implementation

### Basic Tracking Middleware

```go
package mymiddleware

import (
    "fmt"
    "github.com/sipeed/picoclaw/pkg/tools"
)

type TrackingMiddleware struct {
    ToolsAttempted int
    ToolsSucceeded int
    ToolsFailed    int
    ToolNames      []string
    LastError      error
}

func NewTrackingMiddleware() *TrackingMiddleware {
    return &TrackingMiddleware{
        ToolNames: []string{},
    }
}

func (m *TrackingMiddleware) OnToolCall(name string, args map[string]any) {
    m.ToolsAttempted++
    m.ToolNames = append(m.ToolNames, name)
}

func (m *TrackingMiddleware) OnToolResult(name string, result *tools.ToolResult) {
    if result.IsError || result.Err != nil {
        m.ToolsFailed++
        m.LastError = result.Err
    } else {
        m.ToolsSucceeded++
    }
}

func (m *TrackingMiddleware) OnEmptyFinalAnswer(iteration int, maxIterations int) string {
    if m.ToolsAttempted > 0 {
        if m.ToolsFailed > 0 {
            return fmt.Sprintf("I attempted to check %d tool(s) but encountered %d error(s).",
                m.ToolsAttempted, m.ToolsFailed)
        }
        return fmt.Sprintf("I checked %d tool(s) but have nothing specific to report.",
            m.ToolsAttempted)
    }
    return "" // Use default response
}
```

### Custom Empty Response Handler

```go
package mymiddleware

import "fmt"

type CustomResponseMiddleware struct {
    ToolsAttempted int
}

func (m *CustomResponseMiddleware) OnToolCall(name string, args map[string]any) {
    m.ToolsAttempted++
}

func (m *CustomResponseMiddleware) OnToolResult(name string, result *tools.ToolResult) {
    // Not needed for this use case
}

func (m *CustomResponseMiddleware) OnEmptyFinalAnswer(iteration int, maxIterations int) string {
    // Provide different messages based on context
    if m.ToolsAttempted > 0 {
        return "I processed your request through my tools but didn't find anything to report back."
    }
    
    if iteration >= maxIterations {
        return fmt.Sprintf("I reached the maximum number of iterations (%d) without completing.",
            maxIterations)
    }
    
    return "I'm not sure how to respond to that. Could you rephrase your question?"
}
```

### Force Extra Iteration on Empty Response

```go
package mymiddleware

import (
    "github.com/sipeed/picoclaw/pkg/providers"
)

type ForceIterationMiddleware struct{}

func NewForceIterationMiddleware() *ForceIterationMiddleware {
    return &ForceIterationMiddleware{}
}

func (m *ForceIterationMiddleware) OnToolCall(name string, args map[string]any) {}

func (m *ForceIterationMiddleware) OnToolResult(name string, result *tools.ToolResult) {}

func (m *ForceIterationMiddleware) OnEmptyFinalAnswer(iteration int, maxIterations int) string {
    return "" // Not used when OnEmptyFinalAnswerExtended is implemented
}

// OnEmptyFinalAnswerExtended requests one more iteration with a system prompt
// when the LLM returns empty content after tools were executed.
func (m *ForceIterationMiddleware) OnEmptyFinalAnswerExtended(
    messages []providers.Message,
    iteration int,
    maxIterations int,
) ([]providers.Message, bool) {
    // Don't force if at iteration limit
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
```

## Default Behavior

When no callbacks are set (`SetCallbacks` not called or called with `nil`), the agent uses built-in defaults:

1. **Tool execution**: No tracking
2. **Empty response after tools**: 
   - Message: `"I checked the requested information but have nothing to report."`
3. **Empty response without tools**:
   - Message: `"I've completed processing but have no response to give. Increase 'max_tool_iterations' in config.json."`

## Notes

- Callbacks are optional. The agent works normally without them.
- Callbacks are called synchronously during the agent loop - keep them fast.
- `OnEmptyFinalAnswer` can return an empty string to fall back to default behavior.
- `OnEmptyFinalAnswerExtended` allows requesting ONE extra iteration maximum (prevents infinite loops).
- If both `OnEmptyFinalAnswerExtended` returns `false` and `OnEmptyFinalAnswer` returns empty, the default response is used.
- Callbacks persist for the lifetime of the `AgentLoop` instance.
- The callbacks interface may be extended in future versions with additional hooks.

## Thread Safety

The callback methods are called from within the agent's goroutine. If your callback needs to be thread-safe (e.g., accessing shared state), you must implement your own synchronization.

```go
type ThreadSafeMiddleware struct {
    mu             sync.RWMutex
    toolsAttempted int
}

func (m *ThreadSafeMiddleware) OnToolCall(name string, args map[string]any) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.toolsAttempted++
}
```
