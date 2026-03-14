# PicoClaw Session and Context Management

This document explains how PicoClaw manages conversation sessions and prepares context for AI interactions.

---

## Table of Contents

1. [Context Preparation](#context-preparation)
2. [Session Management](#session-management)
3. [Context Window and Summarization](#context-window-and-summarization)
4. [Configuration Reference](#configuration-reference)
5. [Best Practices](#best-practices)

---

## Context Preparation

### Overview

For every AI interaction, PicoClaw builds a comprehensive system prompt from multiple sources. This context is divided into **static** (cached) and **dynamic** (per-request) components.

### Context Building Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    SYSTEM PROMPT                            │
├─────────────────────────────────────────────────────────────┤
│ 1. CORE IDENTITY (hardcoded)                                │
│    - Role definition ("You are picoclaw...")                │
│    - Workspace path                                         │
│    - Important rules (ALWAYS use tools, Memory, etc.)       │
├─────────────────────────────────────────────────────────────┤
│ 2. BOOTSTRAP FILES (workspace/)                             │
│    - AGENTS.md    → Agent instructions                      │
│    - SOUL.md      → Personality & values                    │
│    - USER.md      → User preferences                        │
│    - IDENTITY.md  → Agent identity details                  │
├─────────────────────────────────────────────────────────────┤
│ 3. SKILLS SUMMARY                                           │
│    - List of available skills from workspace/skills/        │
│    - Global skills (~/.picoclaw/skills/)                    │
│    - Builtin skills                                         │
├─────────────────────────────────────────────────────────────┤
│ 4. MEMORY                                                   │
│    - Content from workspace/memory/MEMORY.md                │
├─────────────────────────────────────────────────────────────┤
│ 5. DYNAMIC CONTEXT (per-request)                            │
│    - Current time                                           │
│    - Runtime info (OS, Go version)                          │
│    - Current session (channel, chat ID)                     │
├─────────────────────────────────────────────────────────────┤
│ 6. CONVERSATION SUMMARY (if available)                      │
│    - Auto-generated summary of old conversation             │
│    - Shown as CONTEXT_SUMMARY in system prompt              │
└─────────────────────────────────────────────────────────────┘
```

### Core Identity (Hardcoded)

Located in `pkg/agent/context.go`, this is always present and cannot be overridden:

```markdown
# picoclaw 🦞

You are picoclaw, a helpful AI assistant.

## Workspace
Your workspace is at: {absolute_workspace_path}
- Memory: {workspace}/memory/MEMORY.md
- Daily Notes: {workspace}/memory/YYYYMM/YYYYMMDD.md
- Skills: {workspace}/skills/{skill-name}/SKILL.md

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, 
   send messages, execute commands, etc.), you MUST call the appropriate tool.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory** - When interacting with me if something seems memorable, update 
   {workspace}/memory/MEMORY.md

4. **Context summaries** - Conversation summaries may be incomplete or outdated — 
   always defer to explicit user instructions.
```

### Bootstrap Files

These files are loaded from the workspace directory and can be customized:

| File | Purpose | Default Location |
|------|---------|------------------|
| `AGENTS.md` | General agent instructions | `workspace/AGENTS.md` |
| `SOUL.md` | Personality, values, behavior | `workspace/SOUL.md` |
| `USER.md` | User preferences and info | `workspace/USER.md` |
| `IDENTITY.md` | Agent identity details | `workspace/IDENTITY.md` |

**Note**: These files are optional. If missing, they are simply skipped.

### Skills Loading

Skills are discovered from three sources (priority order):

1. **Workspace skills**: `{workspace}/skills/{name}/SKILL.md`
2. **Global skills**: `~/.picoclaw/skills/{name}/SKILL.md`
3. **Builtin skills**: `{cwd}/skills/{name}/SKILL.md`

Only the **skill summary** (name, description, path) is loaded into context. Full skill content must be read using the `read_file` tool.

### Caching

The static context (identity + bootstrap + skills + memory) is **cached** to avoid repeated file I/O:

- Cache is invalidated when any source file changes (mtime check)
- Dynamic context (time, session) is appended fresh each request
- Cache persists across requests but not across restarts

---

## Session Management

### Session Storage

Sessions are stored as JSON files on disk:

| Setting | Value |
|---------|-------|
| **Location** | `{workspace}/sessions/` |
| **Format** | `{channel}_{chatID}.json` (colons replaced with underscores) |
| **Example** | `telegram_123456.json`, `discord_987654321.json` |

### Session Structure

```json
{
  "key": "telegram:123456",
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi there!"}
  ],
  "summary": "User greeted me, I responded...",
  "created": "2026-03-12T10:00:00Z",
  "updated": "2026-03-12T11:30:00Z"
}
```

### Session Persistence

- **Sessions survive restarts**: Loaded from disk on startup
- **No automatic cleanup**: Sessions accumulate indefinitely
- **Auto-save**: After every message exchange

### No Auto-Cleanup

**Important**: There is NO automatic session cleanup based on:
- Age (no TTL)
- Inactivity
- File size
- Message count

Sessions must be manually cleared if needed.

### How to Clear Sessions

#### Option 1: Delete Session Files
```bash
# Delete ALL sessions
rm ~/.picoclaw/workspace/sessions/*.json

# Delete specific session
rm ~/.picoclaw/workspace/sessions/telegram_123456.json
```

#### Option 2: Manual Truncation (Code)
The `TruncateHistory(key, keepLast)` method is available but not exposed via config:

```go
// Keep only last N messages
agent.Sessions.TruncateHistory("telegram:123456", 10)
```

#### Option 3: Restart with Clean Slate
1. Stop the gateway
2. Delete session files
3. Start the gateway

**Note**: Simply restarting does NOT clear sessions (they are reloaded from disk).

---

## Context Window and Summarization

### Context Window Setting

The context window is determined by `max_tokens` configuration:

| Config Path | Environment Variable | Default |
|-------------|---------------------|---------|
| `agents.defaults.max_tokens` | `PICOCLAW_AGENTS_DEFAULTS_MAX_TOKENS` | 32768 |

```json
{
  "agents": {
    "defaults": {
      "max_tokens": 32768
    }
  }
}
```

This sets both:
- `MaxTokens`: Maximum tokens for LLM response
- `ContextWindow`: Maximum tokens for conversation history

### Summarization Threshold (75%)

When conversation history reaches **75%** of the context window, automatic summarization triggers:

```
Threshold = ContextWindow × 0.75

Examples:
- max_tokens: 32768 → threshold: 24,576 tokens
- max_tokens: 8192  → threshold: 6,144 tokens
- max_tokens: 4096  → threshold: 3,072 tokens
```

### Token Estimation

PicoClaw uses a conservative heuristic to estimate tokens:

```
Tokens = CharacterCount × 2 / 5
```

This assumes **2.5 characters per token** to account for:
- CJK (Chinese/Japanese/Korean) characters
- Code and special symbols
- Multi-byte Unicode

### Auto-Summarization Triggers

Summarization occurs when **EITHER** condition is met:

1. **Message count** > 20 messages
2. **Token estimate** > 75% of context window

### What Happens During Summarization

1. **Background process** generates summary of old messages
2. **History is truncated** to last 4 messages
3. **Summary is saved** and included in future system prompts
4. **Original messages** are permanently removed from history

```go
// From loop.go:1182-1186
agent.Sessions.SetSummary(sessionKey, finalSummary)
agent.Sessions.TruncateHistory(sessionKey, 4)  // Keep only last 4
agent.Sessions.Save(sessionKey)
```

### Force Compression (Emergency)

If the context window is exceeded during processing, emergency compression occurs:

```go
// From loop.go:975-1026
// Drops oldest 50% of conversation messages
// Keeps: system prompt + last message
// Adds compression note to system prompt
```

This is a last-resort measure when:
- LLM returns "context_length_exceeded" error
- "maximum context length" error
- "token limit" error

### Conversation Summary in Context

When a summary exists, it appears in the system prompt:

```markdown
CONTEXT_SUMMARY: The following is an approximate summary of prior conversation 
for reference only. It may be incomplete or outdated — always defer to explicit instructions.

[Summary content here...]
```

---

## Configuration Reference

### Session-Related Settings

| Setting | Path | Default | Description |
|---------|------|---------|-------------|
| Workspace | `agents.defaults.workspace` | `~/.picoclaw/workspace` | Base directory for sessions |
| Max Tokens | `agents.defaults.max_tokens` | 32768 | Context window size |
| DM Scope | `session.dm_scope` | `"per-channel-peer"` | How DMs are scoped |

### Media Cleanup (Different from Session Cleanup)

```json
{
  "tools": {
    "media_cleanup": {
      "enabled": true,
      "max_age_minutes": 30,
      "interval_minutes": 5
    }
  }
}
```

**Note**: Media cleanup only affects uploaded images/files, NOT conversation sessions.

### Environment Variables

```bash
# Override max tokens
PICOCLAW_AGENTS_DEFAULTS_MAX_TOKENS=8192

# Override workspace
PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/custom/workspace
```

---

## Best Practices

### Managing Session Size

1. **Monitor session directory size**:
   ```bash
   du -sh ~/.picoclaw/workspace/sessions/
   ls -la ~/.picoclaw/workspace/sessions/
   ```

2. **Clear old sessions periodically** if you have many chats

3. **Adjust max_tokens** based on your needs:
   - Smaller values (4096-8192) for faster responses
   - Larger values (32768+) for complex multi-turn tasks

### Workspace Organization

```
~/.picoclaw/
├── workspace/                    # Default workspace
│   ├── AGENTS.md                # Custom agent instructions
│   ├── SOUL.md                  # Personality definition
│   ├── USER.md                  # User preferences
│   ├── IDENTITY.md              # Agent identity
│   ├── memory/
│   │   └── MEMORY.md            # Long-term memory
│   ├── skills/                  # Custom skills
│   │   └── my-skill/
│   │       └── SKILL.md
│   └── sessions/                # Conversation history
│       ├── telegram_123456.json
│       └── discord_789012.json
└── skills/                      # Global skills (shared)
```

### Troubleshooting

| Issue | Solution |
|-------|----------|
| "Context window exceeded" errors | Increase `max_tokens` or clear old sessions |
| High memory usage | Clear session files and restart |
| Slow responses | Reduce `max_tokens` or clear accumulated history |
| Lost conversation history | Check `sessions/` directory permissions |

---

## Summary

| Feature | Behavior |
|---------|----------|
| **Context Building** | Static (cached) + Dynamic (per-request) |
| **Context Sources** | Core identity, bootstrap files, skills, memory |
| **Session Storage** | JSON files in `{workspace}/sessions/` |
| **Session Persistence** | Survives restarts, no auto-cleanup |
| **Context Window** | Set by `max_tokens` (default: 32768) |
| **Summarization Trigger** | >20 messages OR >75% of context window |
| **Post-Summary State** | Last 4 messages + summary retained |
| **Emergency Compression** | Drops 50% of oldest messages on overflow |

---

*For more information, see the main [README.md](../README.md) and source code in `pkg/agent/` and `pkg/session/`.*
