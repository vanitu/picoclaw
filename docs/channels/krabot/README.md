# Krabot Channel

Krabot is a WebSocket-based chat channel designed for human users to interact with PicoClaw. It provides a simple, real-time chat interface with support for typing indicators, message editing, and media file handling via signed URLs.

## Overview

Krabot is ideal for:
- Custom web chat interfaces
- Human-in-the-loop AI interactions
- Applications requiring direct user communication with agents

## 1. Example Configuration

Add this to `config.json`:

```json
{
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

### Authentication

Include the token in the `Authorization` header:

```javascript
const ws = new WebSocket('ws://localhost:8080/krabot/ws', [], {
  headers: { Authorization: 'Bearer YOUR_TOKEN' }
});
```

Or via query parameter (less secure):

```javascript
const ws = new WebSocket('ws://localhost:8080/krabot/ws?token=YOUR_TOKEN');
```

### Session Management

Specify a session ID for persistent conversations:

```javascript
const ws = new WebSocket('ws://localhost:8080/krabot/ws?session_id=my-session-123');
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

#### `media.send`
Send media with a signed URL:
```json
{
  "type": "media.send",
  "id": "media-123",
  "payload": {
    "type": "image",
    "url": "https://storage.yourdomain.com/files/abc123?signed=true",
    "filename": "image.jpg",
    "mime_type": "image/jpeg"
  }
}
```

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
  
  sendMedia(type, url, filename, mimeType) {
    this.ws.send(JSON.stringify({
      type: 'media.send',
      id: crypto.randomUUID(),
      payload: { type, url, filename, mime_type: mimeType }
    }));
  }
}

// Usage
const client = new KrabotClient('ws://localhost:8080', 'my-token');
client.connect();
client.sendMessage('Hello, Krabot!');
```

## 8. Security Best Practices

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

## 9. Features

- ✅ Real-time WebSocket communication
- ✅ Session-based persistent conversations
- ✅ Typing indicators
- ✅ Placeholder messages with updates
- ✅ Message editing
- ✅ Media file support via signed URLs
- ✅ CORS support
- ✅ Connection limits
- ✅ Token-based authentication
