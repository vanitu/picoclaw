# Pico Channel WebSocket Protocol

The Pico Channel is the native WebSocket protocol for PicoClaw, designed for real-time bidirectional communication with the AI agent gateway. It serves as the reference implementation for all channel communication patterns in PicoClaw.

## Table of Contents

- [Overview](#overview)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [WebSocket Connection](#websocket-connection)
- [Message Format](#message-format)
- [Client → Server Messages](#client--server-messages)
- [Server → Client Messages](#server--client-messages)
- [Session Management](#session-management)
- [Error Handling](#error-handling)
- [Example Clients](#example-clients)
- [Security Best Practices](#security-best-practices)

## Overview

The Pico Channel protocol enables clients to:

- Send messages to the PicoClaw AI agent
- Receive AI responses in real-time
- Observe typing indicators
- Receive placeholder and updated messages
- Maintain persistent sessions across reconnections

### Key Features

| Feature | Description |
|---------|-------------|
| **Bidirectional** | Real-time full-duplex communication |
| **Session-based** | Persistent conversations via session IDs |
| **Typing Indicators** | Visual feedback when AI is processing |
| **Message Updates** | Support for placeholder → final message flow |
| **Auto-reconnection** | Built-in ping/pong heartbeat |
| **Secure** | Token-based authentication |

## Getting Started

### Prerequisites

1. PicoClaw gateway running with Pico channel enabled
2. Authentication token configured
3. WebSocket client library

### Quick Connection Example

```javascript
const ws = new WebSocket('ws://localhost:8080/pico/ws', [], {
  headers: { Authorization: 'Bearer your-token-here' }
});

ws.onopen = () => {
  console.log('Connected to PicoClaw');
  
  // Send a message
  ws.send(JSON.stringify({
    type: 'message.send',
    payload: { content: 'Hello, PicoClaw!' }
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log('Received:', msg);
};
```

## Configuration

Add the following to your PicoClaw configuration file:

```json
{
  "channels": {
    "pico": {
      "enabled": true,
      "token": "your-secure-token-here",
      "allow_token_query": false,
      "allow_origins": ["https://yourdomain.com"],
      "ping_interval": 30,
      "read_timeout": 60,
      "write_timeout": 10,
      "max_connections": 100,
      "allow_from": [],
      "placeholder": {
        "enabled": true,
        "text": "Thinking... 💭"
      }
    }
  }
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable the Pico channel |
| `token` | string | *(required)* | Authentication token |
| `allow_token_query` | boolean | `false` | Allow token via query parameter |
| `allow_origins` | string[] | `[]` | Allowed CORS origins (`[]` = all) |
| `ping_interval` | integer | `30` | Ping interval in seconds |
| `read_timeout` | integer | `60` | Read timeout in seconds |
| `write_timeout` | integer | `10` | Write timeout in seconds |
| `max_connections` | integer | `100` | Maximum concurrent connections |
| `allow_from` | string[] | `[]` | Allowed sender IDs (empty = all) |
| `placeholder.enabled` | boolean | `true` | Enable placeholder messages |
| `placeholder.text` | string | `"Thinking... 💭"` | Placeholder text |

### Environment Variables

All configuration options can be set via environment variables:

```bash
export PICOCLAW_CHANNELS_PICO_ENABLED=true
export PICOCLAW_CHANNELS_PICO_TOKEN="your-token"
export PICOCLAW_CHANNELS_PICO_ALLOW_FROM="user1,user2"
```

## Authentication

The Pico Channel supports two authentication methods:

### Method 1: Authorization Header (Recommended)

Include the token in the `Authorization` header:

```javascript
const ws = new WebSocket('ws://localhost:8080/pico/ws', [], {
  headers: { Authorization: 'Bearer YOUR_TOKEN' }
});
```

### Method 2: Query Parameter

> ⚠️ **Security Warning**: Only enable this for development or trusted environments.

Requires `allow_token_query: true` in configuration:

```javascript
const ws = new WebSocket('ws://localhost:8080/pico/ws?token=YOUR_TOKEN');
```

## WebSocket Connection

### Endpoint

```
ws://{gateway_host}:{gateway_port}/pico/ws
```

Example: `ws://localhost:8080/pico/ws`

### Session ID

You can specify a session ID for persistent conversations:

```
ws://localhost:8080/pico/ws?session_id=my-session-123
```

If not provided, a UUID is automatically generated.

### Connection Lifecycle

```
┌─────────┐         ┌─────────────┐         ┌──────────┐
│  Client │ ──────▶ │  Handshake  │ ──────▶ │  Server  │
└─────────┘         └─────────────┘         └──────────┘
       │                                          │
       │  Authorization: Bearer <token>           │
       │ ───────────────────────────────────────▶ │
       │                                          │
       │  Connection Accepted                     │
       │ ◀──────────────────────────────────────  │
       │                                          │
       │  [Ping/Pong every 30s]                   │
       │ ◀──────────────────────────────────────▶ │
       │                                          │
       │  message.send                            │
       │ ───────────────────────────────────────▶ │
       │                                          │
       │  message.create / typing.start           │
       │ ◀──────────────────────────────────────  │
       │                                          │
       │  Close                                   │
       │ ◀──────────────────────────────────────▶ │
```

### Connection Limits

- Maximum connections per gateway: `max_connections` (default: 100)
- Excess connections receive HTTP 503 (Service Unavailable)

## Message Format

All messages use JSON format with the following structure:

```typescript
interface PicoMessage {
  type: string;           // Message type identifier
  id?: string;            // Optional message ID for correlation
  session_id?: string;    // Session identifier
  timestamp?: number;     // Unix timestamp in milliseconds
  payload?: {             // Type-specific data
    [key: string]: any;
  };
}
```

### Example Message

```json
{
  "type": "message.send",
  "id": "msg-123",
  "session_id": "session-abc",
  "timestamp": 1708451200000,
  "payload": {
    "content": "Hello, world!"
  }
}
```

## Client → Server Messages

### `message.send`

Send a text message to the AI agent.

```json
{
  "type": "message.send",
  "id": "optional-correlation-id",
  "payload": {
    "content": "Your message here"
  }
}
```

**Payload Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | Yes | Message text content |

**Example:**

```javascript
ws.send(JSON.stringify({
  type: 'message.send',
  payload: { content: 'What is the weather today?' }
}));
```

### `media.send` (Not Implemented)

> ❌ **Not Supported**: This message type is defined in the protocol specification but **not yet implemented**. Sending this message type will result in an `unknown_type` error.

**Planned format (subject to change when implemented):**
```json
{
  "type": "media.send",
  "payload": {
    "type": "image",
    "ref": "media://abc123"
  }
}
```

**Current workaround**: Use other channels (Slack, Discord, Telegram) that support media, or encode small files as base64 in the message content.

### `ping`

Keep-alive ping message. The server responds with `pong`.

```json
{
  "type": "ping",
  "id": "ping-123"
}
```

**Note**: The server also sends WebSocket ping frames automatically every `ping_interval` seconds.

## Server → Client Messages

### `message.create`

New message from the AI agent.

```json
{
  "type": "message.create",
  "session_id": "session-abc",
  "timestamp": 1708451200000,
  "payload": {
    "content": "Hello! How can I help you today?",
    "message_id": "msg-uuid-123"
  }
}
```

### `message.update`

Updated/edited message (typically placeholder replacement).

```json
{
  "type": "message.update",
  "session_id": "session-abc",
  "timestamp": 1708451201000,
  "payload": {
    "content": "Here is the detailed response...",
    "message_id": "msg-uuid-123"
  }
}
```

### `media.create` (Not Implemented)

> ❌ **Not Supported**: This message type is defined but the Pico channel does not implement the `MediaSender` interface. The AI cannot send media files through the Pico channel.

Use other channels (Slack, Discord, Telegram, OneBot) for media support.

### `typing.start`

AI has started processing/generating a response.

```json
{
  "type": "typing.start",
  "session_id": "session-abc",
  "timestamp": 1708451200000
}
```

### `typing.stop`

AI has finished processing.

```json
{
  "type": "typing.stop",
  "session_id": "session-abc",
  "timestamp": 1708451201000
}
```

### `pong`

Response to a client `ping`.

```json
{
  "type": "pong",
  "id": "ping-123",
  "timestamp": 1708451200000
}
```

### `error`

Error message from the server.

```json
{
  "type": "error",
  "timestamp": 1708451200000,
  "payload": {
    "code": "invalid_message",
    "message": "Failed to parse message"
  }
}
```

**Error Codes:**

| Code | Description |
|------|-------------|
| `invalid_message` | JSON parsing failed |
| `empty_content` | Message content is empty |
| `unknown_type` | Unknown message type |

## Session Management

### Session ID Format

- User-provided: Any string (recommended: UUID or semantic ID)
- Auto-generated: UUID v4

### Session Persistence

Messages sent with the same `session_id` are part of the same conversation context:

```javascript
// Connect with specific session
const ws = new WebSocket('ws://localhost:8080/pico/ws?session_id=my-chat-123');

// All messages in this session share conversation history
```

### Chat ID Format

Internally, the gateway uses chat IDs in the format:
```
pico:{session_id}
```

Example: `pico:my-chat-123`

## Error Handling

### Connection Errors

| Error | HTTP Status | Description |
|-------|-------------|-------------|
| Unauthorized | 401 | Invalid or missing token |
| Too Many Connections | 503 | Connection limit exceeded |
| Channel Not Running | 503 | Pico channel is disabled |

### Message Errors

When a message error occurs, the server sends an `error` message:

```javascript
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  
  if (msg.type === 'error') {
    console.error(`Error ${msg.payload.code}: ${msg.payload.message}`);
    // Handle error appropriately
  }
};
```

### Reconnection Strategy

```javascript
class PicoClient {
  constructor(url, token) {
    this.url = url;
    this.token = token;
    this.reconnectDelay = 1000;
    this.maxReconnectDelay = 30000;
  }
  
  connect() {
    this.ws = new WebSocket(this.url, [], {
      headers: { Authorization: `Bearer ${this.token}` }
    });
    
    this.ws.onclose = () => {
      console.log('Connection closed, reconnecting...');
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };
    
    this.ws.onopen = () => {
      this.reconnectDelay = 1000; // Reset on success
    };
  }
}
```

## Example Clients

### JavaScript Client

```javascript
class PicoClient {
  constructor(gatewayUrl, token, sessionId = null) {
    this.gatewayUrl = gatewayUrl;
    this.token = token;
    this.sessionId = sessionId || this.generateId();
    this.ws = null;
    this.messageHandlers = new Map();
  }
  
  generateId() {
    return crypto.randomUUID();
  }
  
  connect() {
    const url = `${this.gatewayUrl}/pico/ws?session_id=${this.sessionId}`;
    
    this.ws = new WebSocket(url, [], {
      headers: { Authorization: `Bearer ${this.token}` }
    });
    
    this.ws.onopen = () => console.log('Connected to PicoClaw');
    
    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      this.handleMessage(msg);
    };
    
    this.ws.onerror = (err) => console.error('WebSocket error:', err);
  }
  
  handleMessage(msg) {
    switch (msg.type) {
      case 'message.create':
        console.log('AI:', msg.payload.content);
        break;
      case 'typing.start':
        console.log('AI is typing...');
        break;
      case 'typing.stop':
        console.log('AI stopped typing');
        break;
      case 'error':
        console.error('Error:', msg.payload);
        break;
    }
  }
  
  sendMessage(content) {
    const msg = {
      type: 'message.send',
      id: this.generateId(),
      payload: { content }
    };
    this.ws.send(JSON.stringify(msg));
  }
  
  ping() {
    this.ws.send(JSON.stringify({ type: 'ping', id: this.generateId() }));
  }
  
  close() {
    this.ws?.close();
  }
}

// Usage
const client = new PicoClient('ws://localhost:8080', 'my-token');
client.connect();
client.sendMessage('Hello, PicoClaw!');
```

### Python Client

```python
import asyncio
import json
import websockets
import uuid

class PicoClient:
    def __init__(self, gateway_url: str, token: str, session_id: str = None):
        self.gateway_url = gateway_url
        self.token = token
        self.session_id = session_id or str(uuid.uuid4())
        self.ws = None
        
    async def connect(self):
        url = f"{self.gateway_url}/pico/ws?session_id={self.session_id}"
        headers = {"Authorization": f"Bearer {self.token}"}
        
        self.ws = await websockets.connect(url, extra_headers=headers)
        print(f"Connected to PicoClaw (session: {self.session_id})")
        
        # Start message handler
        asyncio.create_task(self._receive_loop())
    
    async def _receive_loop(self):
        async for message in self.ws:
            msg = json.loads(message)
            await self._handle_message(msg)
    
    async def _handle_message(self, msg):
        msg_type = msg.get("type")
        
        if msg_type == "message.create":
            print(f"AI: {msg['payload']['content']}")
        elif msg_type == "typing.start":
            print("AI is typing...")
        elif msg_type == "typing.stop":
            print("AI stopped typing")
        elif msg_type == "error":
            print(f"Error: {msg['payload']}")
    
    async def send_message(self, content: str):
        msg = {
            "type": "message.send",
            "id": str(uuid.uuid4()),
            "payload": {"content": content}
        }
        await self.ws.send(json.dumps(msg))
    
    async def close(self):
        await self.ws.close()

# Usage
async def main():
    client = PicoClient("ws://localhost:8080", "my-token")
    await client.connect()
    await client.send_message("Hello, PicoClaw!")
    await asyncio.sleep(10)  # Wait for response
    await client.close()

asyncio.run(main())
```

### Go Client

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

type PicoMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func main() {
	gatewayURL := "ws://localhost:8080"
	token := "your-token"
	sessionID := uuid.New().String()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	url := fmt.Sprintf("%s/pico/ws?session_id=%s", gatewayURL, sessionID)
	ws, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer ws.Close()

	// Handle messages
	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			var msg PicoMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Println("Parse error:", err)
				continue
			}

			switch msg.Type {
			case "message.create":
				fmt.Println("AI:", msg.Payload["content"])
			case "typing.start":
				fmt.Println("AI is typing...")
			case "typing.stop":
				fmt.Println("AI stopped typing")
			case "error":
				fmt.Printf("Error: %v\n", msg.Payload)
			}
		}
	}()

	// Send message
	msg := PicoMessage{
		Type: "message.send",
		ID:   uuid.New().String(),
		Payload: map[string]any{
			"content": "Hello from Go!",
		},
	}

	if err := ws.WriteJSON(msg); err != nil {
		log.Fatal("Write error:", err)
	}

	// Wait for interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
}
```

## Security Best Practices

### Token Management

1. **Use strong tokens**: Generate tokens with at least 32 bytes of entropy
   ```bash
   openssl rand -hex 32
   ```

2. **Rotate tokens regularly**: Change tokens monthly or when compromised

3. **Secure token storage**:
   - Environment variables for server-side
   - Secure storage (Keychain, KeyStore) for clients
   - Never commit tokens to version control

### CORS Configuration

For browser-based clients, configure `allow_origins`:

```json
{
  "channels": {
    "pico": {
      "allow_origins": ["https://app.yourdomain.com"]
    }
  }
}
```

⚠️ **Never use `allow_origins: ["*"]` in production with sensitive tokens.**

### Network Security

1. **Use TLS in production**:
   ```
   wss://your-domain.com/pico/ws
   ```

2. **Firewall rules**: Restrict gateway port access

3. **Rate limiting**: Consider adding rate limiting for production use

### Access Control

Use `allow_from` to restrict which users can access the channel:

```json
{
  "channels": {
    "pico": {
      "allow_from": ["user1", "user2", "admin"]
    }
  }
}
```

---

## Protocol Reference Summary

| Direction | Type | Purpose |
|-----------|------|---------|
| C → S | `message.send` | Send text message |
| C → S | `media.send` | Send media (**not implemented**) |
| C → S | `ping` | Keep-alive |
| S → C | `message.create` | New AI message |
| S → C | `message.update` | Edited message |
| S → C | `media.create` | AI media message (**not implemented**) |
| S → C | `typing.start` | AI started processing |
| S → C | `typing.stop` | AI finished processing |
| S → C | `pong` | Ping response |
| S → C | `error` | Error notification |
