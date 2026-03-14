# Krabot Channel

A human-friendly WebSocket chat channel with ActiveStorage media support.

## Features

- **WebSocket-based** - Real-time bidirectional communication
- **Media support** - ActiveStorage signed URLs for file handling
- **Session management** - Persistent chat sessions
- **Typing indicators** - Visual feedback during processing
- **Simple protocol** - Easy to integrate with web/mobile apps

## Configuration

```json
{
  "channels": {
    "krabot": {
      "enabled": true,
      "token": "your-secret-token",
      "allow_origins": ["https://yourapp.com"],
      "max_connections": 100,
      "allow_from": [],
      "active_storage": {
        "base_url": "https://your-rails-app.com",
        "api_key": "rails-api-key",
        "default_expiry": 3600
      },
      "max_file_size": 10485760,
      "allowed_types": ["image/jpeg", "image/png", "image/gif"]
    }
  }
}
```

## WebSocket Protocol

### Connection

```
WS /krabot/ws?session_id=optional-session-id
Authorization: Bearer <token>
```

### Client → Server

**Send message:**
```json
{
  "type": "message.send",
  "id": "msg-123",
  "payload": {
    "content": "Hello!",
    "media": [{
      "type": "image",
      "url": "https://cdn.signed-url...",
      "filename": "photo.jpg"
    }]
  }
}
```

**Ping:**
```json
{
  "type": "ping",
  "id": "ping-1"
}
```

### Server → Client

**Message received:**
```json
{
  "type": "message.create",
  "session_id": "session-abc",
  "timestamp": 1708451200000,
  "payload": {
    "content": "Hello! How can I help?"
  }
}
```

**Media generated:**
```json
{
  "type": "media.create",
  "session_id": "session-abc",
  "payload": {
    "media": [{
      "type": "image",
      "url": "https://cdn.signed-url...",
      "filename": "generated.png"
    }]
  }
}
```

**Typing indicators:**
```json
{ "type": "typing.start", "session_id": "session-abc" }
{ "type": "typing.stop", "session_id": "session-abc" }
```

## Media Flow

### Client uploads file:
1. Client uploads to ActiveStorage directly (via Rails API)
2. Client gets signed URL from Rails
3. Client sends signed URL to Krabot via WebSocket
4. Krabot downloads from signed URL for AI processing

### AI generates file:
1. AI generates file (image, audio, etc.)
2. Krabot uploads to ActiveStorage
3. Krabot generates signed URL
4. Krabot sends signed URL to client via WebSocket
5. Client displays/downloads from URL

## Environment Variables

- `PICOCLAW_CHANNELS_KRABOT_ENABLED`
- `PICOCLAW_CHANNELS_KRABOT_TOKEN`
- `PICOCLAW_CHANNELS_KRABOT_AS_BASE_URL`
- `PICOCLAW_CHANNELS_KRABOT_AS_API_KEY`
