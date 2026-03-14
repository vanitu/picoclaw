# A2A Channel (Agent-to-Agent Protocol)

The A2A Channel implements Google's Agent-to-Agent protocol, enabling PicoClaw to communicate with other AI agents in a standardized way. This channel supports both synchronous JSON-RPC requests and streaming WebSocket connections.

## Overview

A2A is designed for:
- Agent-to-agent communication
- Multi-agent systems
- External agent integration
- Task delegation and orchestration

## 1. Example Configuration

Add this to `config.json`:

```json
{
  "channels": {
    "a2a": {
      "enabled": true,
      "port": 8080,
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

## 2. Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| enabled | bool | Yes | Enable the A2A channel |
| port | int | No | HTTP port (default: same as gateway) |
| token | string | No | Authentication token for requests |
| agent_card | object | Yes | Agent metadata and capabilities |
| agent_card.name | string | Yes | Agent name |
| agent_card.description | string | Yes | Agent description |
| agent_card.version | string | Yes | Agent version |
| agent_card.capabilities | object | Yes | Agent capabilities |
| agent_card.capabilities.streaming | bool | Yes | Support streaming responses |
| agent_card.capabilities.pushNotifications | bool | Yes | Support push notifications |
| agent_card.capabilities.stateTransitionHistory | bool | Yes | Track task state history |
| agent_card.defaultInputModes | []string | Yes | Supported input modes (e.g., `["text"]`) |
| agent_card.defaultOutputModes | []string | Yes | Supported output modes (e.g., `["text"]`) |
| agent_card.skills | []object | No | Agent skills catalog |
| max_tasks | int | No | Maximum concurrent tasks (default: 100) |
| task_timeout | int | No | Task timeout in seconds (default: 300) |
| max_tasks_per_client | int | No | Max tasks per client (default: 10) |
| enable_streaming | bool | No | Enable WebSocket streaming (default: true) |

## 3. Environment Variables

```bash
export PICOCLAW_CHANNELS_A2A_ENABLED=true
export PICOCLAW_CHANNELS_A2A_PORT=8080
export PICOCLAW_CHANNELS_A2A_TOKEN="your-secure-token"
```

## 4. Endpoints

### Agent Card Discovery

```
GET /.well-known/agent.json
```

Returns the agent's metadata and capabilities:

```json
{
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
      "description": "General conversation",
      "tags": ["conversation"]
    }
  ]
}
```

### JSON-RPC Endpoint

```
POST /a2a
```

Main endpoint for agent communication using JSON-RPC 2.0.

### Streaming Endpoint

```
WS /a2a/stream
```

WebSocket endpoint for real-time streaming communication.

## 5. JSON-RPC Methods

### tasks/send

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

### tasks/sendSubscribe

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

Streaming response events:

```json
{"jsonrpc": "2.0", "id": 2, "result": {"id": "task-456", "status": {"state": "working"}}}
{"jsonrpc": "2.0", "id": 2, "result": {"id": "task-456", "status": {"state": "working"}, "artifacts": [{"parts": [{"type": "text", "text": "Once upon a time..."}]}]}}
{"jsonrpc": "2.0", "id": 2, "result": {"id": "task-456", "status": {"state": "completed"}}}
```

### tasks/get

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

### tasks/cancel

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

## 6. WebSocket Streaming

Connect to the streaming endpoint for real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/a2a/stream');

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

## 7. Authentication

### HTTP Endpoints

Include the token in the `Authorization` header:

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "tasks/send", ...}'
```

### WebSocket

Send authentication message after connection:

```json
{
  "type": "auth",
  "token": "your-token"
}
```

## 8. Task Lifecycle

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

### Task States

| State | Description |
|-------|-------------|
| `submitted` | Task received, waiting to start |
| `working` | Agent is processing the task |
| `completed` | Task finished successfully |
| `failed` | Task encountered an error |
| `cancelled` | Task was cancelled by user |

## 9. Message Parts

A2A messages support multiple part types:

### Text Part

```json
{
  "type": "text",
  "text": "Hello, world!"
}
```

### File Part

```json
{
  "type": "file",
  "file": {
    "name": "document.pdf",
    "mimeType": "application/pdf",
    "bytes": "base64-encoded-content"
  }
}
```

### Data Part

```json
{
  "type": "data",
  "data": {
    "key": "value",
    "number": 42
  }
}
```

## 10. Error Handling

### JSON-RPC Errors

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

### Error Codes

| Code | Message | Description |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC request |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Server internal error |
| -32001 | Task not found | Task ID does not exist |
| -32002 | Task already exists | Duplicate task ID |
| -32003 | Invalid task state | Cannot perform action on current state |

## 11. Python Client Example

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
    
    def get_task(self, task_id: str):
        """Get task status."""
        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tasks/get",
            "params": {"id": task_id}
        }
        resp = requests.post(
            f"{self.base_url}/a2a",
            headers=self.headers,
            json=payload
        )
        return resp.json()
    
    def cancel_task(self, task_id: str):
        """Cancel a running task."""
        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tasks/cancel",
            "params": {"id": task_id}
        }
        resp = requests.post(
            f"{self.base_url}/a2a",
            headers=self.headers,
            json=payload
        )
        return resp.json()

# Usage
client = A2AClient("http://localhost:8080", "your-token")

# Get agent info
card = client.get_agent_card()
print(f"Agent: {card['name']} - {card['description']}")

# Send a task
result = client.send_task("task-001", "Hello, agent!")
print(f"Response: {result['result']['artifacts'][0]['parts'][0]['text']}")
```

## 12. Features

- ✅ Full A2A protocol compliance
- ✅ JSON-RPC 2.0 support
- ✅ WebSocket streaming
- ✅ Task lifecycle management
- ✅ Session persistence
- ✅ Multiple message part types (text, file, data)
- ✅ Agent card discovery
- ✅ Error handling with standard codes
- ✅ Authentication support
- ✅ Concurrent task execution
