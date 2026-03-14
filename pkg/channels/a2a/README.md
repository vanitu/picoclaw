# A2A Channel

Google A2A (Agent-to-Agent) Protocol implementation for PicoClaw.

## Features

- **A2A Protocol v1.0 Compliant** - Standard agent communication
- **JSON-RPC + HTTP** - Standard protocol bindings
- **Task Management** - Stateful task lifecycle
- **WebSocket Streaming** - Real-time task updates
- **Agent Discovery** - Agent Card for capability discovery

## Configuration

```json
{
  "channels": {
    "a2a": {
      "enabled": true,
      "token": "your-secret-token",
      "agent_card": {
        "name": "PicoClaw Agent",
        "description": "AI assistant with A2A support",
        "version": "1.0.0",
        "capabilities": {
          "streaming": true,
          "pushNotifications": false,
          "stateTransitionHistory": true
        },
        "defaultInputModes": ["text/plain", "application/json"],
        "defaultOutputModes": ["text/plain", "application/json"],
        "skills": [
          {
            "id": "chat",
            "name": "Chat",
            "description": "Text-based conversation"
          }
        ]
      },
      "max_tasks": 1000,
      "task_timeout": 30,
      "enable_streaming": true
    }
  }
}
```

## Endpoints

### Discovery

```
GET /.well-known/agent.json
GET /a2a/agent-card
```

Returns the Agent Card with capabilities and skills.

### JSON-RPC

```
POST /a2a
Authorization: Bearer <token>
Content-Type: application/json
```

### Task Operations

```
GET    /a2a/tasks/:id
POST   /a2a/tasks/:id/cancel
```

### WebSocket Streaming

```
WS /a2a/stream
Authorization: Bearer <token>
```

## Protocol

### Send Message

```bash
curl -X POST https://host/a2a \
  -H 'Authorization: Bearer token' \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "method": "sendMessage",
    "params": {
      "message": {
        "role": "user",
        "parts": [
          { "type": "text", "text": "Generate a report" }
        ]
      }
    },
    "id": 1
  }'
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "task": {
      "id": "task-123",
      "status": { "state": "working" }
    }
  },
  "id": 1
}
```

### WebSocket Streaming

```javascript
const ws = new WebSocket('wss://host/a2a/stream', [], {
  headers: { Authorization: 'Bearer token' }
});

// Subscribe to task updates
ws.send(JSON.stringify({
  type: 'subscribe',
  id: 'sub-1',
  payload: { taskId: 'task-123' }
}));

// Receive updates
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  // msg.type: 'task' | 'statusUpdate' | 'artifactUpdate'
};
```

## Task Lifecycle

```
submitted → working → completed
    ↓          ↓
 rejected   failed
    ↓
 canceled
```

## References

- [A2A Protocol Specification](https://a2a-protocol.org)
- [Google A2A Announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/)
