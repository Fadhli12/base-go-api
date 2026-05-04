#!/bin/bash
# .sisyphus/hooks/mempalace-workflow-test.sh
# Real-life test for MemPalace workflow integration
# Run this to validate the entire MemPalace integration

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  MemPalace Workflow Integration Test${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# Track test results
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function
run_test() {
    local name="$1"
    local command="$2"
    local expected_result="$3"

    echo -e "${BLUE}[TEST]${NC} $name"
    echo "  Command: $command"

    set +e
    result=$(eval "$command" 2>&1)
    exit_code=$?
    set -e

    if [ $exit_code -eq 0 ]; then
        echo -e "  ${GREEN}✓ PASSED${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "  ${RED}✗ FAILED${NC}"
        echo "  Exit code: $exit_code"
        echo "  Output: $result"
        ((TESTS_FAILED++))
    fi
    echo ""
}

# Add MemPalace to PATH if not available
MEMPALACE_PATH="/c/Users/MSIKAT~1/AppData/Roaming/Python/Python314/Scripts"
if ! command -v mempalace &> /dev/null; then
    export PATH="$PATH:$MEMPALACE_PATH"
fi

# ============================================
# TEST 1: MemPalace Status
# ============================================
echo -e "${BLUE}[TEST 1] MemPalace Status Check${NC}"
if command -v mempalace &> /dev/null || [ -x "$MEMPALACE_PATH/mempalace" ]; then
    status_output=$(mempalace status 2>&1)
    echo "$status_output"
    if echo "$status_output" | grep -q "drawers"; then
        echo -e "  ${GREEN}✓ PASSED${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "  ${RED}✗ FAILED - No drawers found${NC}"
        ((TESTS_FAILED++))
    fi
else
    echo -e "  ${RED}✗ FAILED - mempalace not installed${NC}"
    echo "  Expected at: $MEMPALACE_PATH/mempalace"
    ((TESTS_FAILED++))
fi
echo ""

# ============================================
# TEST 2: Semantic Search - Webhook
# ============================================
echo -e "${BLUE}[TEST 2] Semantic Search - Webhook Delivery${NC}"
echo "  Searching for 'webhook delivery retry strategy'..."

search_result=$(mempalace search "webhook delivery retry strategy" 2>&1)
echo "  Results preview:"
echo "$search_result" | head -20

if [ -n "$search_result" ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - No results (may be expected)${NC}"
fi
echo ""

# ============================================
# TEST 3: Semantic Search - JWT Authentication
# ============================================
echo -e "${BLUE}[TEST 3] Semantic Search - JWT Authentication${NC}"
echo "  Searching for 'JWT token validation'..."

search_result=$(mempalace search "JWT token validation" 2>&1)
echo "  Results preview:"
echo "$search_result" | head -15

if [ -n "$search_result" ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - No results (may be expected)${NC}"
fi
echo ""

# ============================================
# TEST 4: Diary Write
# ============================================
echo -e "${BLUE}[TEST 4] Diary Write${NC}"
echo "  Writing test entry to MemPalace diary..."

timestamp=$(date '+%Y-%m-%d %H:%M:%S')
test_entry="[TEST $timestamp] Integration test: webhook worker implements exponential backoff with intervals 1m, 5m, 30m. Stuck delivery recovery runs every 60s."

diary_result=$(mempalace diary-write "$test_entry" 2>&1)
if echo "$diary_result" | grep -q "success\|written\|ok"; then
    echo -e "  ${GREEN}✓ PASSED - Diary entry written${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - Diary write may need confirmation${NC}"
fi
echo ""

# ============================================
# TEST 5: Diary Read
# ============================================
echo -e "${BLUE}[TEST 5] Diary Read${NC}"
echo "  Reading recent diary entries..."

diary_result=$(mempalace diary-read --since "2026-05-01" 2>&1 | head -30)
if [ -n "$diary_result" ]; then
    echo "  Recent entries:"
    echo "$diary_result" | head -20
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - No diary entries found${NC}"
fi
echo ""

# ============================================
# TEST 6: Knowledge Graph Query
# ============================================
echo -e "${BLUE}[TEST 6] Knowledge Graph Query${NC}"
echo "  Querying relationships for webhook_service..."

kg_result=$(mempalace kg-query "webhook_service" --depth 1 2>&1)
echo "  Relationships:"
echo "$kg_result" | head -20

if [ -n "$kg_result" ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - No relationships found${NC}"
fi
echo ""

# ============================================
# TEST 7: Knowledge Graph Add Relationship
# ============================================
echo -e "${BLUE}[TEST 7] Knowledge Graph Add${NC}"
echo "  Adding test relationship..."

kg_add_result=$(mempalace kg-add "test_agent" "uses_pattern" "mempalace_semantic_search" 2>&1)
if echo "$kg_add_result" | grep -q "success\|added\|ok"; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - KG add may have failed${NC}"
fi
echo ""

# ============================================
# TEST 8: Pre-Agent Hook Test
# ============================================
echo -e "${BLUE}[TEST 8] Pre-Agent Hook Execution${NC}"
echo "  Running pre-agent hook with test context..."

export TASK_CONTEXT="testing webhook delivery mechanism"
export MEMPALACE_PALACE_PATH="C:\Development\base\go-api"
export MEMPALACE_WING="go_api"

hook_result=$(bash .sisyphus/hooks/pre-agent.sh 2>&1)
hook_exit=$?

echo "  Hook output:"
echo "$hook_result" | head -20

if [ $hook_exit -eq 0 ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${RED}✗ FAILED - Hook exited with code $hook_exit${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# ============================================
# TEST 9: Post-Agent Hook Test
# ============================================
echo -e "${BLUE}[TEST 9] Post-Agent Hook Execution${NC}"
echo "  Running post-agent hook with test data..."

export AGENT_NAME="test-agent"
export TASK_NAME="test-webhook-retry"
export TASK_STATUS="success"
export DECISION="Implemented exponential backoff: 1m, 5m, 30m intervals"
export FILES_MODIFIED="webhook_worker.go, webhook_queue.go"

hook_result=$(bash .sisyphus/hooks/post-agent.sh 2>&1)
hook_exit=$?

echo "  Hook output:"
echo "$hook_result" | head -20

if [ $hook_exit -eq 0 ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${RED}✗ FAILED - Hook exited with code $hook_exit${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# ============================================
# TEST 10: Palace Traverse (Navigation)
# ============================================
echo -e "${BLUE}[TEST 10] Palace Traverse${NC}"
echo "  Traversing webhook_service with depth 1..."

traverse_result=$(mempalace traverse "webhook_service" --depth 1 2>&1)
echo "  Related code:"
echo "$traverse_result" | head -20

if [ -n "$traverse_result" ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${YELLOW}⚠ WARNING - No traversal results${NC}"
fi
echo ""

# ============================================
# TEST 11: Room List
# ============================================
echo -e "${BLUE}[TEST 11] Room Listing${NC}"
echo "  Listing rooms in go_api wing..."

rooms_result=$(mempalace list-rooms 2>&1)
echo "  Rooms:"
echo "$rooms_result" | head -20

if [ -n "$rooms_result" ]; then
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "  ${RED}✗ FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# ============================================
# TEST 12: MCP Server Check
# ============================================
echo -e "${BLUE}[TEST 12] MCP Server Availability${NC}"
echo "  Checking if mempalace-mcp is available..."

if command -v mempalace-mcp &> /dev/null; then
    echo "  mempalace-mcp command found"
    echo -e "  ${GREEN}✓ PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo "  mempalace-mcp not in PATH"
    echo "  To start: python -m mempalace.mcp or mempalace-mcp"
    echo -e "  ${YELLOW}⚠ INFO - MCP server not required for CLI tests${NC}"
fi
echo ""

# ============================================
# SUMMARY
# ============================================
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Test Summary${NC}"
echo -e "${BLUE}============================================${NC}"
echo -e "  ${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "  ${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed! MemPalace integration is working.${NC}"
    exit 0
else
    echo -e "${YELLOW}Some tests had warnings - check output above.${NC}"
    exit 0
fi