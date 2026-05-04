# .sisyphus/hooks/pre-agent.ps1
# PowerShell wrapper for pre-agent hook (Windows-friendly)

param(
    [string]$TaskContext = "",
    [string]$AgentName = "unknown"
)

# Configuration
$MEMPALACE_PALACE_PATH = $env:MEMPALACE_PALACE_PATH -or "C:\Development\base\go-api"
$MEMPALACE_WING = $env:MEMPALACE_WING -or "go_api"

Write-Host "[MemPalace Pre-Agent Hook]" -ForegroundColor Blue
Write-Host "Agent: $AgentName"
Write-Host "Task Context: $TaskContext"

# Only search if context is provided
if ([string]::IsNullOrWhiteSpace($TaskContext)) {
    Write-Host "No task context provided - skipping MemPalace search"
    exit 0
}

# Search MemPalace for relevant context
Write-Host "Searching MemPalace for context..." -ForegroundColor Blue

# Try to search - fail gracefully if MemPalace not available
$mempalaceCmd = Get-Command mempalace -ErrorAction SilentlyContinue
if ($mempalaceCmd) {
    try {
        $searchResults = & mempalace search $TaskContext 2>$null
        
        if ($searchResults) {
            Write-Host "Found relevant context:" -ForegroundColor Green
            $searchResults | Select-Object -First 20 | Write-Host
            
            # Export for agent to use
            $env:MEMPALACE_CONTEXT = $searchResults
            Write-Host "MEMPALACE_CONTEXT exported for agent"
        } else {
            Write-Host "No results found for: $TaskContext"
        }
    } catch {
        Write-Host "Error searching MemPalace: $_"
    }
} else {
    Write-Host "MemPalace not installed - skipping search"
}

Write-Host "Pre-agent hook complete" -ForegroundColor Green
