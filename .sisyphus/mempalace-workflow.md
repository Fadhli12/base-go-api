# MemPalace Workflow Integration Guide

Quick reference for integrating MemPalace into your agent workflow.

## Quick Start

### 1. Start MCP Server
```bash
mempalace-mcp
# Server running on port 3000
```

### 2. Register with Claude Code
```bash
claude mcp add mempalace -- mempalace-mcp
```

### 3. Use in Agent Prompts
```
Before implementing webhook retry logic, search MemPalace for:
- Existing retry patterns in the codebase
- Previous decisions about backoff strategies
- Related webhook implementations

Use MCP tool: mempalace_search("webhook retry exponential backoff")
```

## Hook Integration

### Pattern: Pre-Agent Context Loading
```bash
# Before agent task
export TASK_CONTEXT="webhook delivery retry"
export AGENT_NAME="webhook-retry-impl"
bash .sisyphus/hooks/pre-agent.sh

# Agent now has MEMPALACE_CONTEXT exported with relevant findings
```

### Pattern: Post-Agent Decision Filing
```bash
# After agent task completes
export TASK_STATUS="success"
export DECISION="Exponential backoff: 1m, 5m, 30m"
export FILES_MODIFIED="webhook_worker.go"
bash .sisyphus/hooks/post-agent.sh

# Decision automatically filed to MemPalace diary
```

### Pattern: Full Session Lifecycle
```bash
# Start session
bash .sisyphus/hooks/session-start.sh

# Load context
export TASK_CONTEXT="your search query"
bash .sisyphus/hooks/pre-agent.sh

# Run agent implementation
# ... agent work ...

# File decision
export TASK_STATUS="success"
export DECISION="your decision"
bash .sisyphus/hooks/post-agent.sh

# End session
export SESSION_NOTES="Implementation complete"
bash .sisyphus/hooks/session-end.sh
```

## MCP Tools Reference

### Search Tools
```bash
# Semantic search
mempalace_search("webhook retry strategy")

# Exact search
mempalace_search("webhook_worker.go", exact=true)

# Search by room
mempalace_search("retry", room="internal/service")
```

### Drawer Tools
```bash
# Get drawer details
mempalace_get_drawer("webhook_service")

# Add drawer
mempalace_add_drawer("webhook_retry_decision", "Exponential backoff: 1m, 5m, 30m")

# Update drawer
mempalace_update_drawer("webhook_retry_decision", "Updated: Added jitter to backoff")

# Delete drawer
mempalace_delete_drawer("webhook_retry_decision")
```

### Knowledge Graph Tools
```bash
# Query relationships
mempalace_kg_query("webhook_service")

# Add relationship
mempalace_kg_add("webhook_service", "uses", "event_bus")

# Invalidate cache
mempalace_kg_invalidate("webhook_service")
```

### Navigation Tools
```bash
# Traverse dependencies
mempalace_traverse("webhook_service", depth=2)

# Find tunnels (cross-project links)
mempalace_tunnels("webhook_service")
```

### Diary Tools
```bash
# Write diary entry
mempalace_diary_write("Implemented webhook retry with exponential backoff")

# Read diary
mempalace_diary_read(limit=10)
```

## Environment Variables

```bash
# Required
export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"

# Optional
export MEMPALACE_WING="go_api"
export MEMPALACE_LOG_LEVEL="info"
export MEMPALACE_MCP_PORT="3000"

# Hook-specific
export TASK_CONTEXT="your search query"
export AGENT_NAME="agent-identifier"
export TASK_STATUS="success|failed"
export DECISION="decision made"
export FILES_MODIFIED="file1.go, file2.go"
export SESSION_NOTES="session summary"
```

## Common Workflows

### Workflow 1: Implement New Feature
```bash
# 1. Load context
export TASK_CONTEXT="webhook delivery implementation patterns"
bash .sisyphus/hooks/pre-agent.sh

# 2. Review MEMPALACE_CONTEXT for existing patterns
# 3. Implement feature following patterns
# 4. File decision
export TASK_STATUS="success"
export DECISION="Used existing queue pattern from webhook_queue.go"
bash .sisyphus/hooks/post-agent.sh
```

### Workflow 2: Debug Issue
```bash
# 1. Search for similar issues
mempalace_search("webhook delivery timeout")

# 2. Check knowledge graph
mempalace_kg_query("webhook_worker")

# 3. Review diary for past decisions
mempalace_diary_read(limit=20)

# 4. File debugging notes
mempalace_diary_write("Debugged webhook timeout: issue was rate limiter threshold")
```

### Workflow 3: Refactor Code
```bash
# 1. Load context on existing patterns
export TASK_CONTEXT="webhook service architecture"
bash .sisyphus/hooks/pre-agent.sh

# 2. Check dependencies
mempalace_traverse("webhook_service", depth=3)

# 3. Implement refactoring
# 4. File architectural decision
export DECISION="Extracted rate limiter to separate interface for testability"
bash .sisyphus/hooks/post-agent.sh
```

## Troubleshooting

### MCP Server Not Starting
```bash
# Check if mempalace-mcp is installed
which mempalace-mcp

# Try full path
python -m mempalace.mcp

# Check logs
mempalace-mcp --log-level debug
```

### Search Returns No Results
```bash
# Verify palace status
mempalace status

# Try broader search
mempalace_search("webhook")

# Check palace initialization
mempalace repair
```

### Hook Scripts Fail
```bash
# Make executable
chmod +x .sisyphus/hooks/*.sh

# Run with bash explicitly
bash .sisyphus/hooks/pre-agent.sh

# Check environment
echo $MEMPALACE_PALACE_PATH
echo $TASK_CONTEXT
```

### Unicode/Encoding Issues (Windows)
```powershell
# Set encoding
$env:PYTHONIOENCODING = "utf-8"

# Run hook
bash .sisyphus/hooks/pre-agent.sh
```

## Next Steps

1. ✅ Start MCP server: `mempalace-mcp`
2. ✅ Register with Claude Code: `claude mcp add mempalace -- mempalace-mcp`
3. ✅ Test semantic search: `mempalace_search("webhook")`
4. ✅ Try pre-agent hook: `bash .sisyphus/hooks/pre-agent.sh`
5. ⏭️ Integrate into agent tasks
6. ⏭️ Use in implementation workflows

## Files Reference

- `.sisyphus/mcp-config.json` - MCP server configuration
- `.sisyphus/mcp-setup.md` - Detailed MCP setup guide
- `.sisyphus/mempalace-integration.md` - Complete integration guide
- `.sisyphus/mempalace-queries.md` - Semantic search query reference
- `.sisyphus/hooks/pre-agent.sh` - Context loading hook
- `.sisyphus/hooks/post-agent.sh` - Decision filing hook
- `.sisyphus/hooks/session-start.sh` - Session initialization
- `.sisyphus/hooks/session-end.sh` - Session cleanup
- `mempalace.yaml` - Palace configuration (read-only)

## Documentation Links

- [MemPalace Official](https://mempalaceofficial.com)
- [MCP Specification](https://modelcontextprotocol.io)
- [Claude Code MCP Integration](https://claude.ai/docs/mcp)
- [Go API AGENTS.md](./AGENTS.md) - Project agent instructions
