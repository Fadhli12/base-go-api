# MemPalace Integration Guide for Go API

**Status**: ✅ Initialized (8,035 drawers across 14 rooms)
**Test**: ✅ Real-life workflow test passed (`.sisyphus/hooks/mempalace-test.ps1`)
**Updated**: 2026-05-04

---

## Quick Start

### 1. Verify MemPalace Installation (Windows PowerShell)
```powershell
cd C:\Development\base\go-api
$env:PYTHONIOENCODING = "utf-8"
& "C:\Users\MSIKAT~1\AppData\Roaming\Python\Python314\Scripts\mempalace.exe" status
# Expected: 8,035 drawers in go_api wing
```

### 2. Quick Search (Windows Git Bash / PowerShell)
```bash
# Search commands use QUOTED positional argument (NOT space-separated)
mempalace search "webhook retry"                    # ✓ CORRECT
mempalace search webhook retry                     # ✗ WRONG

# On Windows, set encoding first
export PYTHONIOENCODING=utf-8
mempalace search "JWT token refresh"
```

### 2. Start MCP Server (for Claude Code)
```bash
mempalace-mcp
# Runs on default socket/port - Claude Code will auto-detect
```

### 3. Add to Claude Code
```bash
claude mcp add mempalace -- mempalace-mcp
```

---

## Palace Structure

Your MemPalace is organized as:
- **Wing**: `go_api` (project container)
- **14 Rooms** (by directory):
  - `internal` (2,020 drawers) - Core business logic
  - `migrations` (1,702 drawers) - Database migrations
  - `testing` (1,335 drawers) - Test suites
  - `documentation` (1,275 drawers) - Specs, guides
  - `planning` (849 drawers) - Plans, tasks
  - `bin` (197 drawers) - Scripts, tools
  - `general` (185 drawers) - Misc files
  - `backend` (122 drawers) - API-specific
  - `scripts` (92 drawers) - Automation
  - `configuration` (86 drawers) - Config files
  - `templates` (76 drawers) - Code templates
  - `cmd` (66 drawers) - CLI commands
  - `pkg` (20 drawers) - Packages
  - `storage` (10 drawers) - Data storage

**Drawers** = individual files indexed with semantic embeddings

---

## Workflow Patterns

### Pattern A: Agent Context Loading (Before Implementation)

**Goal**: Load relevant context before starting a task

```bash
# Before implementing webhook retry logic:
mempalace search "webhook delivery retry strategy"
# Returns: webhook_worker.go, webhook_queue.go, backoff implementation

# Before adding auth middleware:
mempalace search "JWT token validation middleware"
# Returns: auth.go, jwt_handler.go, permission checks

# Before modifying RBAC:
mempalace search "Casbin enforcer permission check"
# Returns: enforcer.go, permission validation patterns
```

**Integration**: Agents should search MemPalace BEFORE reading code files. This gives semantic context instead of blind file navigation.

### Pattern B: Memory Filing (After Implementation)

**Goal**: Record decisions and learnings for future agents

```bash
# After implementing a feature:
mempalace diary-write "Implemented webhook retry with exponential backoff: 1m, 5m, 30m intervals. Stuck delivery recovery runs every 60s. See webhook_worker.go:142"

# After discovering a pattern:
mempalace kg-add "webhook_service" "uses_pattern" "event_bus_pub_sub"

# After resolving a bug:
mempalace diary-write "Fixed: JWT refresh token wasn't invalidating old tokens. Solution: Add token_version to claims. See auth.go:89"
```

**Integration**: After each agent task completes, file the decision/learning to MemPalace. This compounds knowledge across sessions.

### Pattern C: Cross-Project Navigation (Tunnels)

**Goal**: Link related code across modules

```bash
# Create tunnel from webhook module to event system:
mempalace create-tunnel "webhook_service" "event_bus"

# Navigate to related code:
mempalace traverse "webhook_service" --depth 2
# Returns: EventBus, domain/webhook_events.go, service/webhook_dispatch.go
```

**Integration**: Use tunnels to understand module dependencies and find related implementations.

---

## Semantic Search Examples

### Authentication & JWT
```bash
mempalace search "How do we handle JWT token refresh?"
mempalace search "Where is JWT validation implemented?"
mempalace search "How do we generate access tokens?"
mempalace search "Where are refresh tokens stored?"
mempalace search "How do we handle token expiration?"
```

### RBAC & Permissions
```bash
mempalace search "How does Casbin RBAC work?"
mempalace search "Where is permission validation implemented?"
mempalace search "How do we check user permissions?"
mempalace search "Where are roles defined?"
mempalace search "How do we enforce organization scoping?"
```

### Webhooks
```bash
mempalace search "What's the webhook delivery retry strategy?"
mempalace search "How do we sign webhook payloads?"
mempalace search "Where is webhook rate limiting implemented?"
mempalace search "How do we handle stuck deliveries?"
mempalace search "What's the webhook event dispatch flow?"
```

### Logging & Monitoring
```bash
mempalace search "How do we structure log output?"
mempalace search "Where is request logging implemented?"
mempalace search "How do we propagate context in logs?"
mempalace search "What fields are automatically logged?"
mempalace search "How do we configure log outputs?"
```

### Error Handling
```bash
mempalace search "How do we structure error responses?"
mempalace search "Where are custom errors defined?"
mempalace search "How do we handle validation errors?"
mempalace search "Where is error mapping implemented?"
mempalace search "How do we return HTTP error codes?"
```

### Database & Repositories
```bash
mempalace search "How do we implement soft deletes?"
mempalace search "Where are repositories implemented?"
mempalace search "How do we structure GORM queries?"
mempalace search "Where are migrations organized?"
mempalace search "How do we handle database transactions?"
```

### Testing
```bash
mempalace search "How do we structure integration tests?"
mempalace search "Where are test fixtures defined?"
mempalace search "How do we use testcontainers?"
mempalace search "Where are unit tests organized?"
mempalace search "How do we mock services?"
```

### Architecture & Design
```bash
mempalace search "What's the overall architecture?"
mempalace search "How do we structure domain services?"
mempalace search "Where is the middleware chain defined?"
mempalace search "How do we handle graceful shutdown?"
mempalace search "What's the initialization order?"
```

---

## MCP Tools Reference

### Palace Read Tools
- `status` - Show palace statistics (8,035 drawers, 14 rooms)
- `list_wings` - List all wings (go_api)
- `list_rooms` - List rooms in wing (internal, migrations, etc.)
- `search` - Semantic search across palace
- `get_drawer` - Read specific file content

### Palace Write Tools
- `add_drawer` - Add new file to palace
- `update_drawer` - Update existing file
- `delete_drawer` - Remove file from palace

### Knowledge Graph Tools
- `kg_query` - Query knowledge graph (relationships between code)
- `kg_add` - Add relationship (e.g., "webhook_service uses event_bus")
- `kg_invalidate` - Clear cached relationships

### Navigation Tools
- `traverse` - Navigate related code (follow dependencies)
- `find_tunnels` - Find cross-module connections
- `create_tunnel` - Link related modules

### Agent Diary Tools
- `diary_write` - Record decision/learning
- `diary_read` - Read past decisions

---

## Integration with .sisyphus/ Workflow

### Before Agent Task
```bash
# In .sisyphus/hooks/pre-agent.sh
mempalace search "$TASK_CONTEXT"
# Inject results into agent prompt
```

### After Agent Task
```bash
# In .sisyphus/hooks/post-agent.sh
mempalace diary-write "Task: $TASK_NAME | Decision: $DECISION | Files: $FILES_MODIFIED"
```

### Session Initialization
```bash
# In .sisyphus/hooks/session-start.sh
export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"
export MEMPALACE_WING="go_api"
mempalace status
```

---

## Troubleshooting

### Unicode Encoding Error (Windows)
**Error**: `UnicodeEncodeError: 'charmap' codec can't encode characters`

**Solution** (set Python encoding before running mempalace):
```powershell
# PowerShell - set before running mempalace
$env:PYTHONIOENCODING = "utf-8"
& "C:\Users\MSIKAT~1\AppData\Roaming\Python\Python314\Scripts\mempalace.exe" search "webhook"

# Git Bash - export before running
export PYTHONIOENCODING=utf-8
mempalace search "webhook"
```

**Why**: Windows console (cp1252) can't display Unicode box-drawing characters (─, │, ├, etc.) that MemPalace uses for output formatting. The actual data retrieval works correctly.

### MCP Server Not Found
**Error**: `claude mcp add mempalace -- mempalace-mcp` fails

**Solution**:
```bash
# Verify MemPalace installed
pip show mempalace

# Verify MCP command available
mempalace-mcp --help

# Try full path
python -m mempalace.mcp
```

### Palace Repair
**Error**: Palace corrupted or inconsistent

**Solution**:
```bash
mempalace repair
mempalace compress
mempalace status
```

### Search Returns No Results
**Issue**: Semantic search not finding relevant code

**Solution**:
1. Try broader search: `"webhook"` instead of `"webhook retry exponential backoff"`
2. Use room-specific search: `mempalace search "retry" --room internal`
3. Check palace is indexed: `mempalace status` should show 8,035 drawers

---

## Best Practices

### 1. Search Before Reading
Always search MemPalace before opening files. Semantic search is faster than manual navigation.

### 2. File Decisions Regularly
After each significant decision, write to diary:
```bash
mempalace diary-write "Decision: Use Redis for webhook queue instead of in-memory. Reason: Persistence across restarts. See webhook_queue_redis.go"
```

### 3. Create Tunnels for Dependencies
Link related modules:
```bash
mempalace create-tunnel "webhook_service" "event_bus"
mempalace create-tunnel "permission_enforcer" "rbac_cache"
```

### 4. Use Knowledge Graph for Architecture
Query relationships:
```bash
mempalace kg-query "webhook_service" --depth 2
# Shows: EventBus → WebhookService → WebhookWorker → Redis
```

### 5. Leverage Agent Diary
Read past decisions before implementing similar features:
```bash
mempalace diary-read --since "2026-04-01" --query "retry"
```

---

## Integration with Claude Code

### Setup
1. Install MemPalace: `pip install mempalace`
2. Initialize palace: `mempalace init` (already done)
3. Start MCP server: `mempalace-mcp`
4. Add to Claude Code: `claude mcp add mempalace -- mempalace-mcp`

### Usage in Claude Code
Claude Code will have access to 29 MCP tools. Use them in prompts:

```
Before implementing webhook retry, search MemPalace for "webhook delivery retry strategy" 
to understand existing patterns. Then check the knowledge graph for related services.
```

Claude Code will automatically:
- Search MemPalace for context
- Query knowledge graph for relationships
- Navigate to related code via tunnels
- File decisions to diary after completion

---

## Integration with Other AI Clients

### Gemini CLI
```bash
gemini mcp add mempalace -- mempalace-mcp
```

### Local Models (Ollama, LM Studio)
```bash
# Start MCP server
mempalace-mcp --socket /tmp/mempalace.sock

# Connect local model to socket
ollama serve --mcp-socket /tmp/mempalace.sock
```

### Generic MCP Client
```bash
# Start MCP server
mempalace-mcp --port 3000

# Connect via HTTP
curl http://localhost:3000/tools
```

---

## References

- **Official Docs**: https://mempalaceofficial.com
- **MCP Spec**: https://modelcontextprotocol.io
- **Project Docs**: `C:\Development\base\go-api\AGENTS.md`
- **Palace Config**: `C:\Development\base\go-api\mempalace.yaml`

---

## Next Steps

1. ✅ Verify MemPalace status: `mempalace status`
2. ✅ Start MCP server: `mempalace-mcp`
3. ✅ Add to Claude Code: `claude mcp add mempalace -- mempalace-mcp`
4. ✅ Create and run workflow test: `.sisyphus/hooks/mempalace-test.ps1`
5. ✅ Update docs with correct command syntax (quoted positional arguments)
6. ⏭️ File initial decisions to diary
7. ⏭️ Use semantic search before implementing features

---

## Test Script

Run the real-life workflow test anytime:
```powershell
powershell -ExecutionPolicy Bypass -File .sisyphus/hooks/mempalace-test.ps1
```

**Tests performed**:
- Status check (8,035 drawers confirmed)
- Semantic search (webhook, JWT, logging)
- Diary write/read
- Pre-agent hook execution
- Post-agent hook execution
- Room list
- Knowledge graph query

**Note**: Some commands show Unicode errors on Windows console but data retrieval works correctly. Set `PYTHONIOENCODING=utf-8` to suppress.
