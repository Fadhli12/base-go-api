#!/bin/bash
# .sisyphus/hooks/session-start.sh
# Initialize MemPalace context at session start

set -e

# Configuration
MEMPALACE_PALACE_PATH="${MEMPALACE_PALACE_PATH:-C:\Development\base\go-api}"
MEMPALACE_WING="${MEMPALACE_WING:-go_api}"
SESSION_ID="${SESSION_ID:-$(uuidgen 2>/dev/null || echo 'unknown')}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}[MemPalace Session Start]${NC}"
echo "Palace: $MEMPALACE_PALACE_PATH"
echo "Wing: $MEMPALACE_WING"
echo "Session: $SESSION_ID"

# Export for child processes
export MEMPALACE_PALACE_PATH
export MEMPALACE_WING
export SESSION_ID

# Check if MemPalace is available
if ! command -v mempalace &> /dev/null; then
    echo -e "${YELLOW}MemPalace not installed${NC}"
    exit 0
fi

# Verify palace is initialized
echo -e "${BLUE}Verifying palace...${NC}"

if mempalace status &> /dev/null; then
    STATUS=$(mempalace status 2>/dev/null | head -3)
    echo -e "${GREEN}Palace status:${NC}"
    echo "$STATUS"
else
    echo -e "${YELLOW}Could not verify palace status${NC}"
fi

# Check if MCP server is running
echo -e "${BLUE}Checking MCP server...${NC}"

if command -v mempalace-mcp &> /dev/null; then
    echo -e "${GREEN}MCP server available${NC}"
    echo "Start with: mempalace-mcp"
else
    echo -e "${YELLOW}MCP server not found${NC}"
fi

echo -e "${GREEN}Session initialization complete${NC}"
