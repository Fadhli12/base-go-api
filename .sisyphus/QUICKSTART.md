# MemPalace Integration - Quick Start (5 Minutes)

## Step 1: Verify Installation (30 seconds)

```bash
# Check MemPalace is installed
mempalace status

# Expected output:
# Palace: go_api
# Drawers: 8,035
# Rooms: 14
# Status: Ready
```

## Step 2: Start MCP Server (1 minute)

```bash
# Terminal 1: Start MCP server
mempalace-mcp

# Expected output:
# MemPalace MCP Server
# Listening on port 3000
# Palace: C:\Development\base\go-api
# Wing: go_api
```

## Step 3: Register with Claude Code (1 minute)

```bash
# Terminal 2: Register MCP with Claude Code
claude mcp add mempalace -- mempalace-mcp

# Expected output:
# MCP server registered: mempalace
# Available tools: 29
```

## Step 4: Test Semantic Search (1 minute)

```bash
# Terminal 2: Test search
mempalace search "webhook retry"

# Expected output:
# Found 5 results:
# - webhook_worker.go: Delivery processor with retry logic
# - webhook_queue.go: Queue implementation
# - webhook_rate_limiter.go: Rate limiting
# - webhook_service.go: Service layer
# - webhook.go: Domain entity
```

## Step 5: Try Pre-Agent Hook (1 minute)

```bash
# Terminal 2: Load context before agent task
export TASK_CONTEXT="webhook delivery retry"
bash .sisyphus/hooks/pre-agent.sh

# Expected output:
# [MemPalace Pre-Agent Hook]
# Agent: unknown
# Task Context: webhook delivery retry
# Searching MemPalace for context...
# Found relevant context:
# - webhook_worker.go: Delivery processor
# - webhook_queue.go: Queue implementation
# MEMPALACE_CONTEXT exported for agent
```

---

## Now You're Ready!

### Use in Agent Tasks

**Pattern 1: Load context before implementing**
```bash
export TASK_CONTEXT="webhook delivery implementation"
bash .sisyphus/hooks/pre-agent.sh

# Agent now has MEMPALACE_CONTEXT with relevant findings
# Use in agent prompt: "Review MEMPALACE_CONTEXT for existing patterns"
```

**Pattern 2: File decision after implementing**
```bash
export TASK_STATUS="success"
export DECISION="Implemented exponential backoff: 1m, 5m, 30m"
export FILES_MODIFIED="webhook_worker.go, webhook_queue.go"
bash .sisyphus/hooks/post-agent.sh

# Decision automatically filed to MemPalace diary
```

**Pattern 3: Full session lifecycle**
```bash
bash .sisyphus/hooks/session-start.sh
export TASK_CONTEXT="webhook retry"
bash .sisyphus/hooks/pre-agent.sh
# ... agent implementation ...
export TASK_STATUS="success"
export DECISION="Exponential backoff implemented"
bash .sisyphus/hooks/post-agent.sh
export SESSION_NOTES="Webhook retry complete"
bash .sisyphus/hooks/session-end.sh
```

---

## Common Commands

### Search MemPalace
```bash
mempalace search "webhook"
mempalace search "retry strategy"
mempalace search "event bus"
```

### Query Knowledge Graph
```bash
mempalace_kg_query("webhook_service")
mempalace_traverse("webhook_service", depth=2)
```

### Read Diary
```bash
mempalace_diary_read(limit=10)
```

### Check Palace Status
```bash
mempalace status
```

---

## Troubleshooting

### MCP Server Won't Start
```bash
# Check if port 3000 is in use
netstat -an | findstr :3000

# Try different port
mempalace-mcp --port 3001
```

### Search Returns No Results
```bash
# Try broader search
mempalace search "webhook"

# Check palace health
mempalace status

# Repair if needed
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
```

---

## Documentation

For more details, see:
- `.sisyphus/mempalace-workflow.md` - Workflow patterns
- `.sisyphus/mempalace-integration.md` - Complete guide
- `.sisyphus/mcp-setup.md` - MCP server setup
- `.sisyphus/mempalace-queries.md` - Search queries
- `.sisyphus/VERIFICATION.md` - Verification checklist

---

## Next Steps

1. ✅ Start MCP server: `mempalace-mcp`
2. ✅ Register with Claude Code: `claude mcp add mempalace -- mempalace-mcp`
3. ✅ Test search: `mempalace search "webhook"`
4. ⏭️ Use in agent tasks
5. ⏭️ Monitor diary: `mempalace_diary_read()`

---

**You're all set!** 🎉

Start using MemPalace in your agent workflows now.
