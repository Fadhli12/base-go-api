#!/bin/bash
# .sisyphus/hooks/pre-agent.sh
# Load MemPalace context before agent task starts

set -e

# Configuration
MEMPALACE_PALACE_PATH="${MEMPALACE_PALACE_PATH:-C:\Development\base\go-api}"
MEMPALACE_WING="${MEMPALACE_WING:-go_api}"
AGENT_NAME="${AGENT_NAME:-unknown}"
TASK_CONTEXT="${TASK_CONTEXT:-}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}[MemPalace Pre-Agent Hook]${NC}"
echo "Agent: $AGENT_NAME"
echo "Task Context: $TASK_CONTEXT"

# Only search if context is provided
if [ -z "$TASK_CONTEXT" ]; then
    echo "No task context provided - skipping MemPalace search"
    exit 0
fi

# Search MemPalace for relevant context
echo -e "${BLUE}Searching MemPalace for context...${NC}"

# Try to search - fail gracefully if MemPalace not available
if command -v mempalace &> /dev/null; then
    SEARCH_RESULTS=$(mempalace search "$TASK_CONTEXT" 2>/dev/null || echo "")
    
    if [ -n "$SEARCH_RESULTS" ]; then
        echo -e "${GREEN}Found relevant context:${NC}"
        echo "$SEARCH_RESULTS" | head -20  # Limit output
        
        # Export for agent to use
        export MEMPALACE_CONTEXT="$SEARCH_RESULTS"
        echo "MEMPALACE_CONTEXT exported for agent"
    else
        echo "No results found for: $TASK_CONTEXT"
    fi
else
    echo "MemPalace not installed - skipping search"
fi

echo -e "${GREEN}Pre-agent hook complete${NC}"
