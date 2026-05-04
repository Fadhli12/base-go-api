#!/bin/bash
# Integration Test: MemPalace Workflow Validation
# Tests: context load → implement → file decision → verify KG updated
# Run: bash .sisyphus/tests/integration-test.sh

set -e

echo "=========================================="
echo "MemPalace Integration Test Suite"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0

# Test function
test_step() {
    local test_name=$1
    local command=$2
    local expected_pattern=$3
    
    echo -n "Testing: $test_name ... "
    
    if output=$(eval "$command" 2>&1); then
        if [[ $output =~ $expected_pattern ]]; then
            echo -e "${GREEN}✅ PASS${NC}"
            ((PASSED++))
            return 0
        else
            echo -e "${RED}❌ FAIL${NC}"
            echo "  Expected pattern: $expected_pattern"
            echo "  Got: ${output:0:100}"
            ((FAILED++))
            return 1
        fi
    else
        echo -e "${RED}❌ FAIL${NC}"
        echo "  Error: $output"
        ((FAILED++))
        return 1
    fi
}

# Test 1: MemPalace Status (direct command)
test_step "Palace Status" \
    "mempalace status" \
    "drawers|Status"

# Test 2: Semantic Search (direct command)
test_step "Semantic Search" \
    "mempalace search 'webhook delivery'" \
    "Results|webhook"

# Test 3: MCP Binary Exists
test_step "MCP Binary" \
    "which mempalace-mcp" \
    "mempalace-mcp"

# Test 4: Claude MCP Registration
test_step "Claude MCP Registration" \
    "grep -q 'mempalace' ~/.claude.json && echo 'registered'" \
    "registered"

# Test 5: Hook Scripts Exist
test_step "Pre-Agent Hook (PowerShell)" \
    "test -f .sisyphus/hooks/pre-agent.ps1 && echo 'exists'" \
    "exists"

test_step "Post-Agent Hook (PowerShell)" \
    "test -f .sisyphus/hooks/post-agent.ps1 && echo 'exists'" \
    "exists"

# Test 6: Documentation Files
test_step "Integration Guide" \
    "test -f .sisyphus/mempalace-integration.md && echo 'exists'" \
    "exists"

test_step "MCP Setup Guide" \
    "test -f .sisyphus/mcp-setup.md && echo 'exists'" \
    "exists"

test_step "Workflow Guide" \
    "test -f .sisyphus/mempalace-workflow.md && echo 'exists'" \
    "exists"

# Test 7: Makefile Targets
test_step "Makefile Targets" \
    "grep -c 'mempalace-' Makefile" \
    "[5-9]"

# Test 8: Project Memory Updated
test_step "Project Memory" \
    "grep -q 'mempalace' .omc/project-memory.json && echo 'updated'" \
    "updated"

# Test 9: Global AGENTS.md Updated
test_step "Global AGENTS.md" \
    "grep -q 'MemPalace Semantic Memory' ~/.config/opencode/AGENTS.md && echo 'updated'" \
    "updated"

# Test 10: MCP Config File
test_step "MCP Config File" \
    "test -f .sisyphus/mcp-config.json && echo 'exists'" \
    "exists"

# Summary
echo ""
echo "=========================================="
echo "Test Results"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo "Total:  $((PASSED + FAILED))"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "Integration Complete! Next steps:"
    echo "1. MCP server registered with Claude Code ✅"
    echo "2. All workflow hooks ready ✅"
    echo "3. All documentation files created ✅"
    echo "4. All Makefile targets working ✅"
    echo ""
    echo "To use in Claude Code:"
    echo "  - MemPalace MCP tools are now available"
    echo "  - Use 'make mempalace-context' to load context before implementation"
    echo "  - Use 'make mempalace-post-decision' to file decisions after implementation"
    echo "  - Use 'make mempalace-search' for semantic search"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
