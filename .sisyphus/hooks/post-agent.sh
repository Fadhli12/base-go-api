#!/bin/bash
# .sisyphus/hooks/post-agent.sh
# File agent decisions to MemPalace after task completes

set -e

# Configuration
AGENT_NAME="${AGENT_NAME:-unknown}"
TASK_NAME="${TASK_NAME:-unknown}"
TASK_STATUS="${TASK_STATUS:-unknown}"
DECISION="${DECISION:-}"
FILES_MODIFIED="${FILES_MODIFIED:-}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}[MemPalace Post-Agent Hook]${NC}"
echo "Agent: $AGENT_NAME"
echo "Task: $TASK_NAME"
echo "Status: $TASK_STATUS"

# Only file if task succeeded
if [ "$TASK_STATUS" != "success" ]; then
    echo -e "${YELLOW}Task did not succeed - skipping diary entry${NC}"
    exit 0
fi

# Check if MemPalace is available
if ! command -v mempalace &> /dev/null; then
    echo "MemPalace not installed - skipping diary entry"
    exit 0
fi

# Build diary entry
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
DIARY_ENTRY="[$TIMESTAMP] Agent: $AGENT_NAME | Task: $TASK_NAME"

if [ -n "$DECISION" ]; then
    DIARY_ENTRY="$DIARY_ENTRY | Decision: $DECISION"
fi

if [ -n "$FILES_MODIFIED" ]; then
    DIARY_ENTRY="$DIARY_ENTRY | Files: $FILES_MODIFIED"
fi

# Write to MemPalace diary
echo -e "${BLUE}Filing decision to MemPalace diary...${NC}"

if mempalace diary-write "$DIARY_ENTRY" 2>/dev/null; then
    echo -e "${GREEN}Decision filed successfully${NC}"
else
    echo -e "${YELLOW}Failed to file decision (MemPalace may be unavailable)${NC}"
fi

echo -e "${GREEN}Post-agent hook complete${NC}"
