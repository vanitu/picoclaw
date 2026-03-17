# Krabot Channel

Krabot is a WebSocket-based chat channel designed for human users to interact with PicoClaw. It provides a simple, real-time chat interface with support for typing indicators, message editing, and media file handling via signed URLs.

## Overview

Krabot is ideal for:
- Custom web chat interfaces
- Human-in-the-loop AI interactions
- Applications requiring direct user communication with agents

## Multi-Channel Coexistence

**Important:** Krabot shares the Gateway HTTP server with other webhook-based channels (Pico, A2A, LINE, WeCom, etc.). All channels run on the **same port** (default: 18790) and are routed by path:

| Channel | Path | Example URL |
|---------|------|-------------|
| Pico Protocol | `/pico/*` | `ws://localhost:18790/pico/ws` |
| **Krabot** | `/krabot/*` | `ws://localhost:18790/krabot/ws` |
| A2A | `/a2a/*` | `http://localhost:18790/a2a` |
| A2A Discovery | `/.well-known/agent.json` | `http://localhost:18790/.well-known/agent.json` |

You can enable multiple channels simultaneously in your configuration.

## 1. Example Configuration

Add this to `config.json`:

```json
{
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  },
  "channels": {
    "krabot": {
      "enabled": true,
      "token": "your-secure-token",
      "allow_origins": ["https://yourdomain.com"],
      "max_connections": 100,
      "allow_from": [],
      "active_storage": {
        "base_url": "https://storage.yourdomain.com",
        "api_key": "your-api-key",
        "default_expiry": 3600
      },
      "max_file_size": 10485760,
      "allowed_types": ["image/jpeg", "image/png", "image/gif", "application/pdf"]
    }
  }
}
```

## 2. Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| enabled | bool | Yes | Enable the Krabot channel |
| token | string | Yes | Authentication token for WebSocket connections |
| allow_origins | []string | No | Allowed CORS origins (`[]` = all) |
| max_connections | int | No | Maximum concurrent WebSocket connections (default: 100) |
| allow_from | []string | No | Allowed sender IDs (empty = all) |
| active_storage | object | No | ActiveStorage configuration for file uploads |
| active_storage.base_url | string | No | ActiveStorage service base URL |
| active_storage.api_key | string | No | API key for ActiveStorage authentication |
| active_storage.default_expiry | int | No | Default signed URL expiry in seconds (default: 3600) |
| max_file_size | int64 | No | Maximum file upload size in bytes |
| allowed_types | []string | No | Allowed MIME types for uploads |

## 3. Environment Variables

All configuration options can be set via environment variables:

```bash
export PICOCLAW_CHANNELS_KRABOT_ENABLED=true
export PICOCLAW_CHANNELS_KRABOT_TOKEN="your-secure-token"
export PICOCLAW_CHANNELS_KRABOT_ALLOW_FROM="user1,user2"
export PICOCLAW_CHANNELS_KRABOT_AS_BASE_URL="https://storage.yourdomain.com"
export PICOCLAW_CHANNELS_KRABOT_AS_API_KEY="your-api-key"
export PICOCLAW_CHANNELS_KRABOT_AS_EXPIRY=3600
export PICOCLAW_CHANNELS_KRABOT_MAX_FILE_SIZE=10485760
```

## 4. WebSocket Protocol

### Endpoint

```
ws://{gateway_host}:{gateway_port}/krabot/ws
```

**Default:** `ws://localhost:18790/krabot/ws`

### Authentication

Include the token in the `Authorization` header:

```javascript
const ws = new WebSocket('ws://localhost:18790/krabot/ws', [], {
  headers: { Authorization: 'Bearer YOUR_TOKEN' }
});
```

Or via query parameter (less secure):

```javascript
const ws = new WebSocket('ws://localhost:18790/krabot/ws?token=YOUR_TOKEN');
```

### Session Management

Specify a session ID for persistent conversations:

```javascript
const ws = new WebSocket('ws://localhost:18790/krabot/ws?session_id=my-session-123');
```

If not provided, a UUID is automatically generated.

## 5. Message Format

### Client → Server Messages

#### `message.send`
Send a text message:
```json
{
  "type": "message.send",
  "id": "msg-123",
  "payload": {
    "content": "Hello, Krabot!"
  }
}
```

#### `message.send` with Media (Recommended)
Send a text message with media attachments:
```json
{
  "type": "message.send",
  "id": "msg-123",
  "payload": {
    "content": "What do you see in this image?",
    "media": [
      {
        "type": "image",
        "url": "https://storage.yourdomain.com/files/abc123?signed=true",
        "filename": "image.jpg",
        "content_type": "image/jpeg"
      }
    ]
  }
}
```

#### `media.send`
Send media-only message (no text content):
```json
{
  "type": "media.send",
  "id": "media-123",
  "payload": {
    "media": [
      {
        "type": "image",
        "url": "https://storage.yourdomain.com/files/abc123?signed=true",
        "filename": "image.jpg",
        "content_type": "image/jpeg"
      }
    ]
  }
}
```

**Note:** Media must be wrapped in a `media` array. Field names are:
- `type`: `"image"`, `"audio"`, `"video"`, or `"file"`
- `url`: The signed URL to download the file
- `filename`: The original filename
- `content_type`: The MIME type (e.g., `"image/png"`)

### Server → Client Messages

#### `message.create`
New message from AI:
```json
{
  "type": "message.create",
  "session_id": "session-abc",
  "timestamp": 1708451200000,
  "payload": {
    "content": "Hello! How can I help you?",
    "message_id": "msg-uuid-123"
  }
}
```

#### `message.update`
Updated message (placeholder replacement):
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

#### `typing.start` / `typing.stop`
Typing indicators:
```json
{
  "type": "typing.start",
  "session_id": "session-abc",
  "timestamp": 1708451200000
}
```

## 6. Media Handling

Krabot handles media files via **signed URLs** rather than proxying file data through the gateway. This approach:

- Reduces gateway bandwidth usage
- Allows direct client-to-storage communication
- Supports any storage backend compatible with signed URLs

### Upload Flow

1. Client requests a signed upload URL from your backend
2. Backend creates blob in ActiveStorage and returns signed URL
3. Client uploads file directly to storage
4. Client sends `media.send` message with the signed download URL
5. PicoClaw downloads and processes the file

### Configuration Example (Rails ActiveStorage)

```ruby
# Generate signed URL for upload
def upload_url
  blob = ActiveStorage::Blob.create_before_direct_upload!(
    filename: params[:filename],
    byte_size: params[:size],
    checksum: params[:checksum],
    content_type: params[:content_type]
  )
  
  render json: {
    signed_id: blob.signed_id,
    direct_upload_url: blob.url_for_direct_upload,
    download_url: Rails.application.routes.url_helpers.rails_blob_url(blob)
  }
end
```

## 7. JavaScript Client Example

```javascript
class KrabotClient {
  constructor(gatewayUrl, token, sessionId = null) {
    this.gatewayUrl = gatewayUrl;
    this.token = token;
    this.sessionId = sessionId || crypto.randomUUID();
    this.ws = null;
  }
  
  connect() {
    const url = `${this.gatewayUrl}/krabot/ws?session_id=${this.sessionId}`;
    this.ws = new WebSocket(url, [], {
      headers: { Authorization: `Bearer ${this.token}` }
    });
    
    this.ws.onopen = () => console.log('Connected to Krabot');
    
    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      this.handleMessage(msg);
    };
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
    }
  }
  
  sendMessage(content) {
    this.ws.send(JSON.stringify({
      type: 'message.send',
      id: crypto.randomUUID(),
      payload: { content }
    }));
  }
  
  sendMessageWithMedia(content, mediaItems) {
    // mediaItems: array of {type, url, filename, content_type}
    this.ws.send(JSON.stringify({
      type: 'message.send',
      id: crypto.randomUUID(),
      payload: { content, media: mediaItems }
    }));
  }
  
  sendMedia(type, url, filename, contentType) {
    this.ws.send(JSON.stringify({
      type: 'media.send',
      id: crypto.randomUUID(),
      payload: { 
        media: [{ type, url, filename, content_type: contentType }]
      }
    }));
  }
}

// Usage
const client = new KrabotClient('ws://localhost:18790', 'my-token');
client.connect();

// Send text-only message
client.sendMessage('Hello, Krabot!');

// Send message with media (text + image)
client.sendMessageWithMedia('What do you see?', [
  {
    type: 'image',
    url: 'https://storage.example.com/photo.jpg?signed=true',
    filename: 'photo.jpg',
    content_type: 'image/jpeg'
  }
]);

// Send media-only message
client.sendMedia('image', 'https://storage.example.com/photo.jpg?signed=true', 'photo.jpg', 'image/jpeg');
```

## 8. Error Responses

When something goes wrong, the server sends an error message:

```json
{
  "type": "error",
  "timestamp": 1710859200000,
  "payload": {
    "error": {
      "code": "download_failed",
      "message": "Cannot download given file",
      "details": "dial tcp 127.0.0.1:3000: connect: connection refused",
      "recoverable": false
    }
  }
}
```

### Error Fields

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Machine-readable error code |
| `message` | string | Human-readable error message |
| `details` | string | Technical details for debugging (optional) |
| `recoverable` | boolean | Whether the client can retry (optional) |

### Error Codes

| Code | Description | Recoverable |
|------|-------------|-------------|
| `connection_refused` | Cannot connect to media server | ❌ No |
| `timeout` | Media download timed out | ✅ Yes |
| `not_found` | Media file not found (404) | ❌ No |
| `forbidden` | Access to media denied (403) | ❌ No |
| `server_error` | Media server error (5xx) | ✅ Yes |
| `file_too_large` | Media exceeds size limit | ❌ No |
| `dns_error` | Cannot resolve server address | ✅ Yes |
| `download_failed` | Generic download failure | ❌ No |
| `empty_content` | Message has no text and no media | ❌ No |
| `invalid_message` | JSON parsing failed | ❌ No |
| `unknown_type` | Unrecognized message type | ❌ No |
| `empty_media` | Media message has no items | ❌ No |
| `invalid_media` | No valid media URLs provided | ❌ No |

### Handling Errors

When a media download fails, the **entire message is rejected** — the agent will not receive a partial message. The client should:

1. Display the error message to the user
2. If `recoverable` is `true`, allow retry
3. If `recoverable` is `false`, ask the user to check/fix the issue

Example error handler:

```javascript
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  
  if (msg.type === 'error') {
    const { code, message, recoverable } = msg.payload.error;
    console.error(`Error ${code}: ${message}`);
    
    if (recoverable) {
      showRetryButton();
    } else {
      showErrorToUser(message);
    }
    return;
  }
  
  // Handle other message types...
};
```

## 9. Security Best Practices

1. **Use strong tokens**: Generate tokens with at least 32 bytes of entropy
   ```bash
   openssl rand -hex 32
   ```

2. **Configure CORS properly**: Restrict `allow_origins` to your domain
   ```json
   {
     "allow_origins": ["https://app.yourdomain.com"]
   }
   ```

3. **Use TLS in production**:
   ```
   wss://your-domain.com/krabot/ws
   ```

4. **Validate file uploads**: Configure `max_file_size` and `allowed_types`

5. **Use allow_from for access control**:
   ```json
   {
     "allow_from": ["user1", "user2", "admin"]
   }
   ```

## 10. Features

- ✅ Real-time WebSocket communication
- ✅ Session-based persistent conversations
- ✅ Typing indicators
- ✅ Placeholder messages with updates
- ✅ Message editing
- ✅ Media file support via signed URLs
- ✅ CORS support
- ✅ Connection limits
- ✅ Token-based authentication
- ✅ Multi-channel coexistence (shares port with Pico, A2A, etc.)
