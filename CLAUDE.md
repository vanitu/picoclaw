# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**PicoClaw** is an ultra-lightweight personal AI assistant written in Go that runs on $10 hardware with <10MB RAM. It integrates with multiple chat platforms (Discord, Telegram, Slack, WhatsApp, WeChat, etc.) and uses the Anthropic Claude API as its LLM backbone. The project was substantially developed with AI assistance and embraces this collaborative approach.

## Development Commands

### Build and Installation

```bash
make build              # Build binary for current platform (runs generate first)
make install           # Install to ~/.local/bin
make generate          # Run go generate (regenerates embedded workspace skills)
make clean             # Remove build artifacts
```

### Testing and Code Quality

```bash
make test              # Run all tests
make check             # Full pre-commit check: deps + fmt + vet + test
make lint              # Run golangci-lint
make fix               # Fix linting issues automatically
make fmt               # Format code
make vet               # Static analysis with go vet
```

### Platform-Specific Builds

```bash
make build-all         # Build for all supported platforms
make build-linux-arm   # Build for Raspberry Pi Zero 2 W (32-bit)
make build-linux-arm64 # Build for Raspberry Pi Zero 2 W (64-bit)
make build-pi-zero     # Build both ARM variants
make build-whatsapp-native  # Multi-platform build with WhatsApp native support
```

### Single Test Execution

```bash
go test -run TestName -v ./path/to/package/
go test -bench=. -benchmem -run='^$' ./...  # Run benchmarks
```

## Architecture Overview

### Package Structure

The codebase is organized in `pkg/` with clear separation of concerns:

- **`pkg/agent/`** — Core agent logic, context management, and execution loops. Manages AI conversations and routing
- **`pkg/channels/`** — Integration with external chat platforms (Discord, Telegram, Slack, WhatsApp, etc.). Each platform has its own subdirectory with handlers
- **`pkg/config/`** — Configuration management, environment loading, and settings
- **`pkg/providers/`** — LLM provider abstractions (Anthropic Claude, OpenAI, etc.)
- **`pkg/skills/`** — Skill system for extending agent capabilities
- **`pkg/session/`** — Session persistence and management
- **`pkg/state/`** — State machine and state management
- **`pkg/routing/`** — Message routing between channels and agent
- **`pkg/auth/`** — Authentication and authorization
- **`pkg/bus/`** — Event bus for inter-component communication
- **`pkg/cron/`** — Scheduled task execution
- **`pkg/tools/`** — Hardware interface utilities (I2C, SPI, GPIO, etc.)
- **`pkg/identity/`** — User identity and user profile management
- **`pkg/logger/`** — Structured logging
- **`pkg/media/`** — Media handling and processing
- **`pkg/voice/`** — Voice-related functionality
- **`pkg/health/`** — Health check and monitoring
- **`pkg/heartbeat/`** — Heartbeat and keepalive mechanisms
- **`pkg/migrate/`** — Data migration and schema upgrades
- **`pkg/fileutil/`** — File and path utilities
- **`pkg/utils/`** — General utility functions
- **`pkg/constants/`** — Application constants
- **`pkg/devices/`** — Device-specific handling

### Entry Points

The project has three CLI applications in `cmd/`:

1. **`cmd/picoclaw/`** — Main application with subcommands:
   - `agent` — Run the AI agent
   - `auth` — Authentication management
   - `gateway` — Gateway mode
   - `status` — System status
   - `cron` — Cron job management
   - `migrate` — Data migration
   - `onboard` — Onboarding setup
   - `skills` — Skill management
   - `version` — Version information

2. **`cmd/picoclaw-launcher/`** — Windows launcher application
3. **`cmd/picoclaw-launcher-tui/`** — TUI launcher

### Key Design Patterns

- **Channel abstraction**: Each chat platform implements a common interface for sending/receiving messages
- **Provider pattern**: LLM providers (Claude, OpenAI) implement a pluggable interface
- **Event bus**: Decoupled component communication through event publishing/subscription
- **Workspace pattern**: Embedded binary filesystem for skills and workspace management (see `go generate` usage)

## Code Style and Standards

### Linting Configuration

- **Linter**: golangci-lint with custom configuration in `.golangci.yaml`
- **Code formatters**: goimports, gofumpt, gofmt, golines
- **Max line length**: 120 characters
- **Max function length**: 120 lines / 40 statements

### Testing

- **Framework**: testify (assert, require)
- **Convention**: `*_test.go` files in the same package
- **CI**: All tests must pass before PR merge

## Important Notes

1. **Go version**: Requires Go 1.25 or later
2. **Go generate**: The build system uses `go generate` to embed workspace skills into the binary. This is critical and runs automatically before `make build`
3. **Platform support**: Builds support Linux (amd64, arm, arm64, loong64, riscv64), macOS (arm64), and Windows (amd64)
4. **Dependencies**: Uses Anthropic SDK for Claude integration, managed via go.mod
5. **Configuration**: Managed via environment variables using `github.com/caarlos0/env`
6. **Workspace**: User data is stored in `~/.picoclaw/workspace/` by default

## Contributing

Before making changes:
1. Run `make check` to ensure all tests pass and code is formatted
2. Follow the branch strategy: always branch off and target `main`
3. For substantial features, open an issue first to discuss design
4. See CONTRIBUTING.md for detailed contribution guidelines

## Building for Embedded Systems

PicoClaw is optimized for resource-constrained devices. When working with embedded builds (Raspberry Pi, RISC-V, LoongArch):

- Use `make build-all` for cross-platform compilation
- Test on actual hardware or emulation if possible
- Be mindful of memory constraints when adding features
- The project achieves <10MB RAM usage through careful optimization
