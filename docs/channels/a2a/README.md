# A2A Channel (Agent-to-Agent Protocol)

The A2A Channel implements Google's Agent-to-Agent protocol, enabling PicoClaw to communicate with other AI agents in a standardized way. This channel supports:

1. **A2A Server Mode** - PicoClaw exposes itself as an A2A agent that other agents can call
2. **A2A Client Mode** - PicoClaw can discover and call remote A2A agents

## Overview

A2A is designed for:
- Agent-to-agent communication
- Multi-agent systems
- External agent integration
- Task delegation and orchestration

---

## Table of Contents

1. [A2A Server Mode (Incoming)](#1-a2a-server-mode-incoming)
2. [A2A Client Mode (Outgoing)](#2-a2a-client-mode-outgoing)
3. [Configuration](#3-configuration)
4. [Agent Registry](#4-agent-registry)
5. [Using Remote Agents](#5-using-remote-agents)
6. [Health Monitoring](#6-health-monitoring)
7. [JSON-RPC Protocol](#7-json-rpc-protocol)
8. [WebSocket Streaming](#8-websocket-streaming)
9. [Error Handling](#9-error-handling)
10. [Client Examples](#10-client-examples)

---

## 1. A2A Server Mode (Incoming)

PicoClaw can expose itself as an A2A agent that other agents can discover and call.

### 1.1 Server Configuration

A2A channel uses the shared gateway port (configured via `gateway.port`). No separate port configuration needed.

```json
{
  "gateway": {
    "port": 18790
  },
  "channels": {
    "a2a": {
      "enabled": true,
      "token": "your-secure-token",
      "agent_card": {
        "name": "PicoClaw Agent",
        "description": "A versatile AI agent gateway",
        "version": "1.0.0",
        "capabilities": {
          "streaming": true,
          "pushNotifications": false,
          "stateTransitionHistory": true
        },
        "defaultInputModes": ["text"],
        "defaultOutputModes": ["text"],
        "skills": [
          {
            "id": "chat",
            "name": "Chat",
            "description": "General conversation and task execution",
            "tags": ["conversation", "tasks"],
            "examples": ["Hello!", "What can you do?"]
          }
        ]
      },
      "max_tasks": 100,
      "task_timeout": 300,
      "max_tasks_per_client": 10,
      "enable_streaming": true
    }
  }
}
```

### 1.2 Server Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| enabled | bool | Yes | Enable the A2A channel |
| token | string | No | Authentication token for requests |
| agent_card | object | Yes | Agent metadata and capabilities |
| max_tasks | int | No | Maximum concurrent tasks (default: 100) |
| task_timeout | int | No | Task timeout in seconds (default: 300) |
| max_tasks_per_client | int | No | Max tasks per client (default: 10) |
| enable_streaming | bool | No | Enable WebSocket streaming (default: true) |

**Note:** Port is configured via `gateway.port`, not in the channel config.

### 1.3 Server Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/.well-known/agent.json` | GET | Agent card discovery |
| `/a2a` | POST | JSON-RPC endpoint |
| `/a2a/stream` | WS | WebSocket streaming |

---

## 2. A2A Client Mode (Outgoing)

PicoClaw can discover and call remote A2A agents, delegating tasks to specialized agents.

### 2.1 Client Configuration

Add the `a2a_registry` section to configure remote agents:

```json
{
  "a2a_registry": {
    "agents": [
      {
        "name": "code-reviewer",
        "endpoint": "https://reviewer.example.com",
        "token": "${REVIEWER_TOKEN}",
        "refresh_interval": "1h"
      },
      {
        "name": "translator",
        "endpoint": "https://translator.example.com",
        "token": "${TRANSLATOR_TOKEN}",
        "refresh_interval": "30m"
      },
      {
        "name": "image-generator",
        "endpoint": "https://images.example.com",
        "token": "${IMAGE_TOKEN}"
      }
    ]
  }
}
```

### 2.2 Client Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Unique identifier for the agent |
| endpoint | string | Yes | Base URL of the remote agent |
| token | string | No | Authentication token |
| refresh_interval | string | No | Card refresh interval (default: "1h") |

### 2.3 How It Works

1. **Startup**: PicoClaw fetches agent cards from all configured endpoints
2. **Health Check**: Failed fetches mark agents as "unhealthy" (excluded from context)
3. **Background Refresh**: Periodically refreshes agent cards (every 5 minutes)
4. **Recovery**: Unhealthy agents are retried with exponential backoff
5. **Context Integration**: Only healthy agents appear in the system prompt

---

## 3. Configuration

### 3.1 Environment Variables

```bash
# Gateway port (shared by all channels)
export PICOCLAW_GATEWAY_PORT=18790

# Server settings
export PICOCLAW_CHANNELS_A2A_ENABLED=true
export PICOCLAW_CHANNELS_A2A_TOKEN="your-secure-token"

# Client tokens (referenced in config)
export REVIEWER_TOKEN="token-for-code-reviewer"
export TRANSLATOR_TOKEN="token-for-translator"
export IMAGE_TOKEN="token-for-image-generator"
```

### 3.2 Complete Example

```json
{
  "gateway": {
    "port": 18790
  },
  "channels": {
    "a2a": {
      "enabled": true,
      "token": "${A2A_SERVER_TOKEN}",
      "agent_card": {
        "name": "PicoClaw Gateway",
        "description": "Multi-agent orchestration gateway",
        "version": "1.0.0",
        "capabilities": {
          "streaming": true,
          "pushNotifications": false,
          "stateTransitionHistory": true
        },
        "skills": [
          {
            "id": "orchestration",
            "name": "Agent Orchestration",
            "description": "Delegates tasks to specialized agents",
            "tags": ["delegation", "orchestration"]
          }
        ]
      }
    }
  },
  "a2a_registry": {
    "agents": [
      {
        "name": "code-reviewer",
        "endpoint": "https://reviewer.example.com",
        "token": "${REVIEWER_TOKEN}",
        "refresh_interval": "1h"
      },
      {
        "name": "translator",
        "endpoint": "https://translator.example.com",
        "token": "${TRANSLATOR_TOKEN}",
        "refresh_interval": "1h"
      }
    ]
  }
}
```

---

## 4. Agent Registry

### 4.1 Health States

| State | Description | In Context? |
|-------|-------------|-------------|
| `registered` | Initial state, not yet fetched | No |
| `fetching` | Card fetch in progress | No |
| `healthy` | Card fetched successfully | Yes |
| `unhealthy` | Fetch failed, retry scheduled | No |

### 4.2 Retry Backoff

When an agent fetch fails, retries follow exponential backoff:

| Retry | Backoff Time |
|-------|-------------|
| 1 | 1 minute |
| 2 | 2 minutes |
| 3 | 5 minutes |
| 4+ | 10 minutes (max) |

### 4.3 Logging

All registry operations are logged with structured fields:

```
INFO  a2a  Agent registered  {"agent_name": "code-reviewer", "endpoint": "..."}
INFO  a2a  Agent card fetched  {"agent_name": "code-reviewer", "skills_count": 5}
WARN  a2a  Agent card fetch failed  {"agent_name": "translator", "error": "...", "retry_count": 1}
INFO  a2a  Agent recovered  {"agent_name": "translator", "after_retries": 3}
```

---

## 5. Using Remote Agents

### 5.1 System Prompt Integration

Healthy agents automatically appear in the system prompt:

```
## Available Remote Agents

- **code-reviewer**: Code review specialist (python, go, security)
  Use: call_remote_agent(agent_name="code-reviewer", task="...")
  Details: agent_details(agent_name="code-reviewer")

- **translator**: Translation service (chinese, english, japanese)
  Use: call_remote_agent(agent_name="translator", task="...")
  Details: agent_details(agent_name="translator")
```

### 5.2 Tools

#### `call_remote_agent`

Delegates a task to a remote A2A agent.

**Parameters:**
- `agent_name` (string, required): Name of the agent to call
- `task` (string, required): Task description
- `context` (string, optional): Additional context
- `wait_for_result` (boolean, optional): Wait for completion (default: true)
- `timeout_seconds` (number, optional): Max wait time (default: 60)

**Example:**
```
call_remote_agent(
  agent_name="code-reviewer",
  task="Review this Python function for security issues",
  context="Function handles user input and database queries"
)
```

#### `agent_details`

Retrieves detailed information about a remote agent.

**Parameters:**
- `agent_name` (string, required): Name of the agent

**Example:**
```
agent_details(agent_name="code-reviewer")
```

**Returns:** Full agent card with capabilities, skills, and examples.

### 5.3 Conversation Flow

**Simple delegation (summary sufficient):**
```
User: "Review this Python code"
LLM: [sees code-reviewer in context with skills: python, go, security]
LLM: call_remote_agent(agent_name="code-reviewer", task="Review this Python code...")
→ Returns review results
```

**Complex delegation (need details):**
```
User: "I need specialized security analysis"
LLM: [uncertain which agent fits best]
LLM: agent_details(agent_name="code-reviewer")
→ Returns full capabilities
LLM: [confirmed match]
LLM: call_remote_agent(agent_name="code-reviewer", task="...")
→ Returns analysis
```

**User explicit request:**
```
User: "Ask code-reviewer to check this"
LLM: [parses agent name from user message]
LLM: call_remote_agent(agent_name="code-reviewer", task="...")
→ Returns review
```

---

## 6. Health Monitoring

### 6.1 Automatic Health Checks

- **Startup**: All agents fetched immediately
- **Background**: Refreshed every 5 minutes
- **TTL-based**: Per-agent refresh intervals respected
- **Backoff**: Failed agents retried with exponential backoff

### 6.2 Unhealthy Agent Handling

When an agent becomes unhealthy:
1. Removed from system prompt (invisible to LLM)
2. Logged with error details
3. Scheduled for retry with backoff
4. Automatically re-added on recovery

### 6.3 Monitoring

Check agent status via logs:
```
grep "a2a" /var/log/picoclaw.log
```

Or programmatically (if exposed via API):
```bash
curl http://localhost:18790/api/v1/a2a/agents
```

---

## 7. JSON-RPC Protocol

### 7.1 Methods

#### `tasks/send`

Send a task to the agent:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tasks/send",
  "params": {
    "id": "task-123",
    "sessionId": "session-abc",
    "message": {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "Hello, agent!"
        }
      ]
    }
  }
}
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "task-123",
    "sessionId": "session-abc",
    "status": {
      "state": "completed",
      "timestamp": "2024-01-01T00:00:00Z"
    },
    "artifacts": [
      {
        "parts": [
          {
            "type": "text",
            "text": "Hello! How can I help you today?"
          }
        ]
      }
    ]
  }
}
```

#### `tasks/sendSubscribe`

Subscribe to task updates (returns streaming response):

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tasks/sendSubscribe",
  "params": {
    "id": "task-456",
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Tell me a story"}]
    }
  }
}
```

#### `tasks/get`

Get task status:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tasks/get",
  "params": {
    "id": "task-123"
  }
}
```

#### `tasks/cancel`

Cancel a running task:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tasks/cancel",
  "params": {
    "id": "task-123"
  }
}
```

### 7.2 Task Lifecycle

```
┌─────────┐    send      ┌──────────┐
│  User   │ ───────────▶ │ submitted│
└─────────┘              └────┬─────┘
                              │
                              ▼
                         ┌──────────┐
                    ┌─── │ working  │ ◀── streaming updates
                    │    └────┬─────┘
                    │         │
              cancel│    ┌────┴─────┐
                    └──▶ │completed │
                         │  failed  │
                         │ cancelled│
                         └──────────┘
```

### 7.3 Task States

| State | Description |
|-------|-------------|
| `submitted` | Task received, waiting to start |
| `working` | Agent is processing the task |
| `completed` | Task finished successfully |
| `failed` | Task encountered an error |
| `cancelled` | Task was cancelled by user |

---

## 8. WebSocket Streaming

### 8.1 Connection

Connect to the streaming endpoint for real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:18790/a2a/stream');

ws.onopen = () => {
  // Authenticate
  ws.send(JSON.stringify({
    type: 'auth',
    token: 'your-token'
  }));
  
  // Send a task
  ws.send(JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'tasks/sendSubscribe',
    params: {
      id: 'task-789',
      message: {
        role: 'user',
        parts: [{type: 'text', text: 'Hello!'}]
      }
    }
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log('Received:', msg);
};
```

---

## 9. Error Handling

### 9.1 JSON-RPC Errors

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": {
      "details": "Missing required field: message"
    }
  }
}
```

### 9.2 Error Codes

| Code | Message | Description |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC request |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Server internal error |
| -32001 | Task not found | Task ID does not exist |
| -32002 | Task not cancelable | Cannot cancel in current state |
| -32003 | Content type not supported | Unsupported content type |
| -32004 | Unsupported operation | Operation not supported |

### 9.3 Client-Side Errors

When calling remote agents:

| Error | Cause | Action |
|-------|-------|--------|
| `Agent unavailable` | Agent fetch failed or unhealthy | Check logs, wait for retry |
| `Agent not found` | Agent not in registry | Check configuration |
| `Timeout waiting` | Task didn't complete in time | Increase timeout or retry |
| `Task failed` | Remote agent reported failure | Check task details |

---

## 10. Client Examples

### 10.1 Python Client (Calling PicoClaw)

```python
import requests
import json

class A2AClient:
    def __init__(self, base_url: str, token: str = None):
        self.base_url = base_url.rstrip('/')
        self.token = token
        self.headers = {'Content-Type': 'application/json'}
        if token:
            self.headers['Authorization'] = f'Bearer {token}'
    
    def get_agent_card(self):
        """Fetch agent capabilities."""
        resp = requests.get(f"{self.base_url}/.well-known/agent.json")
        return resp.json()
    
    def send_task(self, task_id: str, message: str, session_id: str = None):
        """Send a task and wait for completion."""
        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tasks/send",
            "params": {
                "id": task_id,
                "message": {
                    "role": "user",
                    "parts": [{"type": "text", "text": message}]
                }
            }
        }
        if session_id:
            payload["params"]["sessionId"] = session_id
        
        resp = requests.post(
            f"{self.base_url}/a2a",
            headers=self.headers,
            json=payload
        )
        return resp.json()

# Usage
client = A2AClient("http://localhost:18790", "your-token")

# Get agent info
card = client.get_agent_card()
print(f"Agent: {card['name']} - {card['description']}")

# Send a task
result = client.send_task("task-001", "Hello, agent!")
print(f"Response: {result['result']['artifacts'][0]['parts'][0]['text']}")
```

### 10.2 cURL Examples

**Fetch agent card:**
```bash
curl http://localhost:18790/.well-known/agent.json
```

**Send a task:**
```bash
curl -X POST http://localhost:18790/a2a \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tasks/send",
    "params": {
      "id": "task-001",
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "Hello!"}]
      }
    }
  }'
```

---

## Features

- ✅ Full A2A protocol compliance
- ✅ Bidirectional agent communication (server + client)
- ✅ Automatic agent discovery and health monitoring
- ✅ Health-based context filtering
- ✅ Exponential backoff for failed agents
- ✅ JSON-RPC 2.0 support
- ✅ WebSocket streaming
- ✅ Task lifecycle management
- ✅ Session persistence
- ✅ Multiple message part types (text, file, data)
- ✅ Agent card discovery
- ✅ Error handling with standard codes
- ✅ Authentication support
- ✅ Concurrent task execution
