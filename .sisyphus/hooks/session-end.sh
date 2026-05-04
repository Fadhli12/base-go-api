#!/bin/bash
# .sisyphus/hooks/session-end.sh
# Clean up and file session notes at session end

set -e

# Configuration
AGENT_NAME="${AGENT_NAME:-unknown}"
SESSION_ID="${SESSION_ID:-unknown}"
SESSION_NOTES="${SESSION_NOTES:-}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}[MemPalace Session End]${NC}"
echo "Agent: $AGENT_NAME"
echo "Session: $SESSION_ID"

# Check if MemPalace is available
if ! command -v mempalace &> /dev/null; then
    echo "MemPalace not installed - skipping cleanup"
    exit 0
fi

# File session summary if provided
if [ -n "$SESSION_NOTES" ]; then
    echo -e "${BLUE}Filing session summary...${NC}"
    
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    SUMMARY="[SESSION END] $TIMESTAMP | Agent: $AGENT_NAME | Session: $SESSION_ID | Notes: $SESSION_NOTES"
    
    if mempalace diary-write "$SUMMARY" 2>/dev/null; then
        echo -e "${GREEN}Session summary filed${NC}"
    else
        echo -e "${YELLOW}Failed to file session summary${NC}"
    fi
fi

# Optional: Compress palace to optimize storage
if [ "${MEMPALACE_AUTO_COMPRESS:-false}" = "true" ]; then
    echo -e "${BLUE}Compressing palace...${NC}"
    
    if mempalace compress 2>/dev/null; then
        echo -e "${GREEN}Palace compressed${NC}"
    else
        echo -e "${YELLOW}Failed to compress palace${NC}"
    fi
fi

echo -e "${GREEN}Session cleanup complete${NC}"
