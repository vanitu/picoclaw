# PicoClaw Docker Support

## Quick Start

```bash
# Build and run
cd docker
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f picoclaw-gateway
```

## Multi-Channel Configuration

**Important:** All webhook-based channels share the **same port** (`18790` by default). The Gateway HTTP server routes requests by path:

| Channel | Path | Full URL Example |
|---------|------|------------------|
| Pico Protocol | `/pico/ws` | `ws://localhost:18790/pico/ws` |
| Krabot WebSocket | `/krabot/ws` | `ws://localhost:18790/krabot/ws` |
| A2A JSON-RPC | `/a2a` | `http://localhost:18790/a2a` |
| A2A Tasks | `/a2a/tasks/:id` | `http://localhost:18790/a2a/tasks/123` |
| A2A Streaming | `/a2a/stream` | `ws://localhost:18790/a2a/stream` |
| A2A Discovery | `/.well-known/agent.json` | `http://localhost:18790/.well-known/agent.json` |
| Health Check | `/health` | `http://localhost:18790/health` |

### Using Multiple Channels Together

Enable multiple channels in your `config.json`:

```json
{
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  },
  "channels": {
    "pico": {
      "enabled": true,
      "token": "your-pico-token"
    },
    "krabot": {
      "enabled": true,
      "token": "your-krabot-token"
    },
    "a2a": {
      "enabled": true,
      "token": "your-a2a-token",
      "agent_card": {
        "name": "My Multi-Channel Agent",
        "description": "Accessible via Pico, Krabot, and A2A"
      }
    }
  }
}
```

All channels work simultaneously on the same port, differentiated by path:
- Pico clients connect to: `ws://your-host:18790/pico/ws`
- Krabot clients connect to: `ws://your-host:18790/krabot/ws`
- A2A clients POST to: `http://your-host:18790/a2a`

## Configuration

Copy `.env.example` to `.env` and customize:

```bash
cp .env.example .env
```

## Docker Compose Variants

### Development (Single Gateway)
```bash
docker compose up -d
```

### Full Stack (Gateway + Agent)
```bash
docker compose -f docker-compose.full.yml up -d
```

### With Web UI Launcher
```bash
docker compose -f docker-compose.full.yml --profile launcher up -d
```

## Building from Source

```bash
# Development image
docker build -f Dockerfile.dev -t picoclaw:dev ..

# Or use make from project root
make docker-build
```

## Ports

| Port | Service | Description |
|------|---------|-------------|
| 18790 | Gateway | Shared by all webhook channels (Pico, Krabot, A2A, etc.) |
| 3000 | Launcher | Web UI (optional, with `--profile launcher`) |

## Volumes

| Path | Description |
|------|-------------|
| `/config` | Configuration files (config.json) |
| `/data` | Data directory (workspace, logs, etc.) |
