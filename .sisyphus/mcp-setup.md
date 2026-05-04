# MemPalace MCP Server Setup

**Status**: Ready to integrate  
**MCP Tools Available**: 29 tools  
**Palace**: 8,035 drawers initialized

---

## Quick Setup (Claude Code)

### Step 1: Verify MemPalace
```bash
mempalace status
# Expected output: 8,035 drawers in go_api wing
```

### Step 2: Start MCP Server
```bash
mempalace-mcp
# Runs on default socket - Claude Code auto-detects
```

### Step 3: Add to Claude Code
```bash
claude mcp add mempalace -- mempalace-mcp
```

### Step 4: Verify in Claude Code
- Open Claude Code settings
- Check "MCP Servers" section
- Should see "mempalace" with 29 tools available

---

## MCP Tools Reference

### Palace Read Tools (5)
| Tool | Purpose | Example |
|------|---------|---------|
| `status` | Show palace statistics | `mempalace status` |
| `list_wings` | List all wings | `mempalace list_wings` |
| `list_rooms` | List rooms in wing | `mempalace list_rooms --wing go_api` |
| `search` | Semantic search | `mempalace search "webhook retry"` |
| `get_drawer` | Read file content | `mempalace get_drawer --path "internal/service/webhook.go"` |

### Palace Write Tools (3)
| Tool | Purpose | Example |
|------|---------|---------|
| `add_drawer` | Add new file | `mempalace add_drawer --path "new_file.go" --content "..."` |
| `update_drawer` | Update file | `mempalace update_drawer --path "file.go" --content "..."` |
| `delete_drawer` | Remove file | `mempalace delete_drawer --path "file.go"` |

### Knowledge Graph Tools (3)
| Tool | Purpose | Example |
|------|---------|---------|
| `kg_query` | Query relationships | `mempalace kg_query "webhook_service"` |
| `kg_add` | Add relationship | `mempalace kg_add "webhook_service" "uses" "event_bus"` |
| `kg_invalidate` | Clear cache | `mempalace kg_invalidate` |

### Navigation Tools (3)
| Tool | Purpose | Example |
|------|---------|---------|
| `traverse` | Navigate dependencies | `mempalace traverse "webhook_service" --depth 2` |
| `find_tunnels` | Find cross-module links | `mempalace find_tunnels "webhook_service"` |
| `create_tunnel` | Link modules | `mempalace create_tunnel "webhook" "event_bus"` |

### Agent Diary Tools (2)
| Tool | Purpose | Example |
|------|---------|---------|
| `diary_write` | Record decision | `mempalace diary_write "Implemented webhook retry"` |
| `diary_read` | Read decisions | `mempalace diary_read --since "2026-04-01"` |

### Utility Tools (13+)
- `mine` - Index new files
- `sweep` - Clean up palace
- `compress` - Optimize storage
- `repair` - Fix corruption
- `split` - Partition palace
- `wake_up` - Activate palace
- `hook` - Setup webhooks
- `instructions` - Show help
- `migrate` - Migrate palace
- And more...

---

## Setup for Different AI Clients

### Claude Code (Recommended)
```bash
# 1. Start MCP server
mempalace-mcp

# 2. Add to Claude Code
claude mcp add mempalace -- mempalace-mcp

# 3. Verify
# Open Claude Code → Settings → MCP Servers → mempalace (should show 29 tools)
```

### Gemini CLI
```bash
# 1. Start MCP server
mempalace-mcp

# 2. Add to Gemini
gemini mcp add mempalace -- mempalace-mcp

# 3. Use in prompts
# "Search MemPalace for webhook implementation"
```

### Local Models (Ollama)
```bash
# 1. Start MCP server on socket
mempalace-mcp --socket /tmp/mempalace.sock

# 2. Connect Ollama
ollama serve --mcp-socket /tmp/mempalace.sock

# 3. Use in local model
# "Use MemPalace tools to search for webhook patterns"
```

### Generic MCP Client
```bash
# 1. Start MCP server on port
mempalace-mcp --port 3000

# 2. Connect client
curl http://localhost:3000/tools

# 3. Call tools via HTTP
curl -X POST http://localhost:3000/tools/search \
  -H "Content-Type: application/json" \
  -d '{"query": "webhook retry"}'
```

---

## Configuration

### Environment Variables
```bash
# Palace location
export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"

# MCP server settings
export MEMPALACE_MCP_PORT=3000
export MEMPALACE_MCP_SOCKET="/tmp/mempalace.sock"

# Logging
export MEMPALACE_LOG_LEVEL="info"
export MEMPALACE_LOG_FILE="/var/log/mempalace.log"

# Performance
export MEMPALACE_CACHE_SIZE="1000"
export MEMPALACE_SEARCH_TIMEOUT="30"
```

### MCP Server Options
```bash
# Custom palace path
mempalace-mcp --palace /path/to/palace

# Custom port
mempalace-mcp --port 3000

# Custom socket
mempalace-mcp --socket /tmp/mempalace.sock

# Verbose logging
mempalace-mcp --verbose

# Disable caching
mempalace-mcp --no-cache
```

---

## Verification Checklist

- [ ] MemPalace installed: `pip show mempalace`
- [ ] Palace initialized: `mempalace status` shows 8,035 drawers
- [ ] MCP server runs: `mempalace-mcp --help` works
- [ ] Claude Code sees MCP: Settings → MCP Servers → mempalace
- [ ] Search works: `mempalace search "webhook"` returns results
- [ ] Tools available: `mempalace status` shows all 29 tools

---

## Troubleshooting

### MCP Server Won't Start
```bash
# Check Python installation
python --version

# Check MemPalace installation
pip show mempalace

# Try full path
python -m mempalace.mcp

# Check port availability
netstat -an | grep 3000
```

### Claude Code Doesn't See MCP
```bash
# 1. Verify MCP server is running
mempalace-mcp

# 2. Check Claude Code settings
# Settings → MCP Servers → Add new

# 3. Try manual add
claude mcp add mempalace -- mempalace-mcp

# 4. Restart Claude Code
```

### Search Returns No Results
```bash
# 1. Check palace is indexed
mempalace status

# 2. Try broader search
mempalace search "webhook"  # instead of specific query

# 3. Re-index palace
mempalace mine

# 4. Check room-specific
mempalace search "webhook" --room internal
```

### Unicode Encoding Error (Windows)
```powershell
# Set Python encoding
$env:PYTHONIOENCODING = "utf-8"

# Or in Command Prompt
set PYTHONIOENCODING=utf-8

# Then run
mempalace-mcp
```

### Palace Corruption
```bash
# Repair palace
mempalace repair

# Check status
mempalace repair-status

# Compress and optimize
mempalace compress

# Verify
mempalace status
```

---

## Usage Examples

### In Claude Code Prompts

**Example 1: Before Implementation**
```
Before implementing webhook retry logic, search MemPalace for 
"webhook delivery retry strategy" to understand existing patterns.
Then query the knowledge graph for related services.
```

**Example 2: Architecture Decision**
```
Query MemPalace knowledge graph to understand how EventBus 
connects to WebhookService. Use traverse to find all dependencies.
```

**Example 3: Bug Investigation**
```
Search MemPalace for "JWT token refresh" to find where tokens 
are validated. Check the diary for past decisions about token expiration.
```

### Command Line

**Search for Implementation**
```bash
mempalace search "How do we implement soft deletes?"
# Returns: domain/user.go, repository/user.go, migrations/...
```

**Query Knowledge Graph**
```bash
mempalace kg_query "webhook_service" --depth 2
# Returns: EventBus → WebhookService → WebhookWorker → Redis
```

**Navigate Dependencies**
```bash
mempalace traverse "webhook_service" --depth 3
# Returns: All services that depend on webhook_service
```

**Record Decision**
```bash
mempalace diary_write "Implemented webhook retry with exponential backoff: 1m, 5m, 30m. See webhook_worker.go:142"
```

**Read Past Decisions**
```bash
mempalace diary_read --since "2026-04-01" --query "retry"
# Returns: All decisions about retry logic since April 1st
```

---

## Performance Tips

1. **Use Specific Searches**: `"webhook delivery retry"` is faster than `"webhook"`
2. **Leverage Rooms**: `mempalace search "query" --room internal` narrows scope
3. **Cache Results**: MCP server caches search results (configurable)
4. **Batch Operations**: Use `traverse` instead of multiple searches
5. **Monitor Performance**: `mempalace status` shows cache hit rate

---

## Integration with Workflow

### Pre-Agent Hook
```bash
#!/bin/bash
# .sisyphus/hooks/pre-agent.sh

# Load context before agent starts
CONTEXT=$(mempalace search "$TASK_CONTEXT")
echo "MEMPALACE_CONTEXT=$CONTEXT" >> agent.env
```

### Post-Agent Hook
```bash
#!/bin/bash
# .sisyphus/hooks/post-agent.sh

# File decision after agent completes
mempalace diary_write "Task: $TASK_NAME | Decision: $DECISION"
```

### Session Initialization
```bash
#!/bin/bash
# .sisyphus/hooks/session-start.sh

export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"
export MEMPALACE_WING="go_api"
mempalace status
```

---

## References

- **Official Docs**: https://mempalaceofficial.com
- **MCP Specification**: https://modelcontextprotocol.io
- **Project Docs**: `C:\Development\base\go-api\AGENTS.md`
- **Palace Config**: `C:\Development\base\go-api\mempalace.yaml`

---

## Next Steps

1. ✅ Verify MemPalace: `mempalace status`
2. ✅ Start MCP server: `mempalace-mcp`
3. ✅ Add to Claude Code: `claude mcp add mempalace -- mempalace-mcp`
4. ⏭️ Test search: `mempalace search "webhook"`
5. ⏭️ Create workflow hooks
6. ⏭️ Start using in agent prompts
