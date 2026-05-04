# MemPalace Workflow Hooks

Automated hooks for integrating MemPalace with your agent workflow.

## Overview

Four hooks automate MemPalace integration:
- **pre-agent.sh** - Load context before agent task
- **post-agent.sh** - File decisions after task completes
- **session-start.sh** - Initialize MemPalace at session start
- **session-end.sh** - Clean up and file summary at session end

## Hook Files

### pre-agent.sh
**When**: Before agent task execution  
**Purpose**: Load relevant MemPalace context and inject into agent prompt

**Environment Variables**:
- `TASK_CONTEXT` - Search query (e.g., "webhook retry strategy")
- `AGENT_NAME` - Agent identifier
- `MEMPALACE_PALACE_PATH` - Palace location

**Output**:
- Exports `MEMPALACE_CONTEXT` with search results
- Logs found context to stdout

**Example**:
```bash
export TASK_CONTEXT="webhook delivery retry strategy"
export AGENT_NAME="webhook-implementation"
bash .sisyphus/hooks/pre-agent.sh
# Searches MemPalace and exports results
```

### post-agent.sh
**When**: After agent task completes  
**Purpose**: File agent decisions to MemPalace diary

**Environment Variables**:
- `AGENT_NAME` - Agent identifier
- `TASK_NAME` - Task description
- `TASK_STATUS` - "success" or "failed"
- `DECISION` - Decision made (optional)
- `FILES_MODIFIED` - Files changed (optional)

**Output**:
- Writes diary entry to MemPalace
- Logs success/failure to stdout

**Example**:
```bash
export AGENT_NAME="webhook-implementation"
export TASK_NAME="Implement webhook retry"
export TASK_STATUS="success"
export DECISION="Use exponential backoff: 1m, 5m, 30m"
export FILES_MODIFIED="webhook_worker.go, webhook_queue.go"
bash .sisyphus/hooks/post-agent.sh
# Files decision to MemPalace diary
```

### session-start.sh
**When**: At session initialization  
**Purpose**: Set up MemPalace context and verify setup

**Environment Variables**:
- `MEMPALACE_PALACE_PATH` - Palace location (default: project root)
- `MEMPALACE_WING` - Wing name (default: go_api)
- `SESSION_ID` - Session identifier (auto-generated if not provided)

**Output**:
- Exports environment variables for child processes
- Verifies palace status
- Checks MCP server availability

**Example**:
```bash
export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"
export MEMPALACE_WING="go_api"
bash .sisyphus/hooks/session-start.sh
# Initializes MemPalace context
```

### session-end.sh
**When**: At session completion  
**Purpose**: File session summary and clean up

**Environment Variables**:
- `AGENT_NAME` - Agent identifier
- `SESSION_ID` - Session identifier
- `SESSION_NOTES` - Summary notes (optional)
- `MEMPALACE_AUTO_COMPRESS` - Auto-compress palace (default: false)

**Output**:
- Files session summary to diary
- Optionally compresses palace
- Logs completion to stdout

**Example**:
```bash
export AGENT_NAME="webhook-implementation"
export SESSION_ID="ses_abc123"
export SESSION_NOTES="Completed webhook retry implementation with tests"
export MEMPALACE_AUTO_COMPRESS="true"
bash .sisyphus/hooks/session-end.sh
# Files summary and compresses palace
```

## Integration Patterns

### Pattern 1: Manual Hook Execution

```bash
# Before agent task
export TASK_CONTEXT="webhook retry strategy"
bash .sisyphus/hooks/pre-agent.sh

# Run agent task
# ... agent implementation ...

# After agent task
export TASK_STATUS="success"
export DECISION="Implemented exponential backoff"
bash .sisyphus/hooks/post-agent.sh
```

### Pattern 2: Automated with Make

Add to `Makefile`:
```makefile
.PHONY: agent-task
agent-task:
	@bash .sisyphus/hooks/session-start.sh
	@bash .sisyphus/hooks/pre-agent.sh
	@echo "Running agent task..."
	# ... agent task command ...
	@bash .sisyphus/hooks/post-agent.sh
	@bash .sisyphus/hooks/session-end.sh
```

Usage:
```bash
TASK_CONTEXT="webhook retry" AGENT_NAME="webhook-impl" make agent-task
```

### Pattern 3: Automated with Shell Script

Create `.sisyphus/run-agent.sh`:
```bash
#!/bin/bash
set -e

TASK_CONTEXT="$1"
AGENT_NAME="$2"

export TASK_CONTEXT AGENT_NAME

bash .sisyphus/hooks/session-start.sh
bash .sisyphus/hooks/pre-agent.sh

# Run agent task
echo "Running agent task..."
# ... agent implementation ...

export TASK_STATUS="success"
export DECISION="Implementation complete"
bash .sisyphus/hooks/post-agent.sh

export SESSION_NOTES="Task completed successfully"
bash .sisyphus/hooks/session-end.sh
```

Usage:
```bash
bash .sisyphus/run-agent.sh "webhook retry strategy" "webhook-impl"
```

## Hook Templates

### Search Template
```bash
# Search MemPalace for context
SEARCH_RESULTS=$(mempalace search "your query here")
echo "Found: $SEARCH_RESULTS"
```

### Diary Template
```bash
# Write to MemPalace diary
ENTRY="[$(date '+%Y-%m-%d %H:%M:%S')] Your decision here"
mempalace diary-write "$ENTRY"
```

### Knowledge Graph Template
```bash
# Add relationship to knowledge graph
mempalace kg-add "service_a" "uses" "service_b"
mempalace kg-add "webhook_service" "depends_on" "event_bus"
```

### Traverse Template
```bash
# Navigate dependencies
mempalace traverse "webhook_service" --depth 2
```

## Environment Variables

### Required
- `MEMPALACE_PALACE_PATH` - Path to MemPalace palace

### Optional
- `MEMPALACE_WING` - Wing name (default: go_api)
- `AGENT_NAME` - Agent identifier
- `SESSION_ID` - Session identifier
- `TASK_CONTEXT` - Search query for pre-agent
- `TASK_NAME` - Task description
- `TASK_STATUS` - Task completion status
- `DECISION` - Decision made
- `FILES_MODIFIED` - Files changed
- `SESSION_NOTES` - Session summary
- `MEMPALACE_AUTO_COMPRESS` - Auto-compress palace (true/false)

## Troubleshooting

### Hooks Not Executing
```bash
# Check permissions
ls -la .sisyphus/hooks/

# Make executable
chmod +x .sisyphus/hooks/*.sh

# Run manually
bash .sisyphus/hooks/pre-agent.sh
```

### MemPalace Not Found
```bash
# Verify installation
pip show mempalace

# Check PATH
which mempalace

# Try full path
python -m mempalace.mcp
```

### Diary Write Fails
```bash
# Check palace status
mempalace status

# Repair if needed
mempalace repair

# Try manual write
mempalace diary-write "Test entry"
```

### Unicode Errors (Windows)
```powershell
# Set encoding
$env:PYTHONIOENCODING = "utf-8"

# Run hook
bash .sisyphus/hooks/pre-agent.sh
```

## Best Practices

1. **Always call session-start first**: Initializes environment
2. **Call pre-agent before task**: Loads context
3. **Call post-agent after task**: Files decision
4. **Call session-end last**: Cleans up
5. **Export variables**: Use `export` for child processes
6. **Check status**: Verify task success before filing decision
7. **Provide context**: More specific queries = better results
8. **Use meaningful names**: Agent names help with diary searches

## Examples

### Example 1: Webhook Implementation
```bash
#!/bin/bash
export TASK_CONTEXT="webhook delivery retry exponential backoff"
export AGENT_NAME="webhook-retry-impl"
export TASK_NAME="Implement webhook retry logic"

bash .sisyphus/hooks/session-start.sh
bash .sisyphus/hooks/pre-agent.sh

# Agent implementation here
echo "Implementing webhook retry..."

export TASK_STATUS="success"
export DECISION="Exponential backoff: 1m, 5m, 30m intervals"
export FILES_MODIFIED="webhook_worker.go, webhook_queue.go"
bash .sisyphus/hooks/post-agent.sh

export SESSION_NOTES="Webhook retry implementation complete with tests"
bash .sisyphus/hooks/session-end.sh
```

### Example 2: Authentication Feature
```bash
#!/bin/bash
export TASK_CONTEXT="JWT token refresh implementation"
export AGENT_NAME="auth-refresh-impl"
export TASK_NAME="Implement JWT refresh token"

bash .sisyphus/hooks/session-start.sh
bash .sisyphus/hooks/pre-agent.sh

# Agent implementation here
echo "Implementing JWT refresh..."

export TASK_STATUS="success"
export DECISION="Added token_version to claims for invalidation"
export FILES_MODIFIED="auth.go, jwt_handler.go"
bash .sisyphus/hooks/post-agent.sh

export SESSION_NOTES="JWT refresh implementation complete"
bash .sisyphus/hooks/session-end.sh
```

## Integration with .sisyphus/ Workflow

Hooks integrate with your existing `.sisyphus/` structure:

```
.sisyphus/
├── hooks/
│   ├── pre-agent.sh
│   ├── post-agent.sh
│   ├── session-start.sh
│   └── session-end.sh
├── notepads/
│   └── {plan-name}/
│       ├── learnings.md
│       ├── issues.md
│       └── decisions.md
├── plans/
│   └── {plan-name}.md
└── mempalace-integration.md
```

Hooks automatically:
- Load context from MemPalace before agent tasks
- File decisions to MemPalace after completion
- Maintain session continuity
- Compress palace for optimization

---

## Next Steps

1. ✅ Make hooks executable: `chmod +x .sisyphus/hooks/*.sh`
2. ✅ Test pre-agent: `bash .sisyphus/hooks/pre-agent.sh`
3. ✅ Test session-start: `bash .sisyphus/hooks/session-start.sh`
4. ⏭️ Integrate with your workflow
5. ⏭️ Use in agent tasks
