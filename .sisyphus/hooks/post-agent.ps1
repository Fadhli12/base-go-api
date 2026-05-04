# .sisyphus/hooks/post-agent.ps1
# PowerShell wrapper for post-agent hook (Windows-friendly)
# Files agent decisions to MemPalace diary after task completes

param(
    [Parameter(Mandatory=$true)]
    [string]$TaskStatus = "unknown",

    [string]$Decision = "",

    [string]$FilesModified = "",

    [string]$SessionNotes = ""
)

# Configuration
$MEMPALACE_PALACE_PATH = $env:MEMPALACE_PALACE_PATH -or "C:\Development\base\go-api"
$MEMPALACE_WING = $env:MEMPALACE_WING -or "go_api"

Write-Host "[MemPalace Post-Agent Hook]" -ForegroundColor Blue
Write-Host "Task Status: $TaskStatus"
Write-Host "Decision: $Decision"
Write-Host "Files Modified: $FilesModified"

# Set encoding for UTF-8
$env:PYTHONIOENCODING = "utf-8"

# Only file decision if status is success
if ($TaskStatus -ne "success") {
    Write-Host "Task status is not 'success' - skipping decision filing" -ForegroundColor Yellow
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Decision)) {
    Write-Host "No decision provided - skipping filing" -ForegroundColor Yellow
    exit 0
}

# Check if mempalace is available
$mempalaceCmd = Get-Command mempalace -ErrorAction SilentlyContinue

if (-not $mempalaceCmd) {
    Write-Host "MemPalace not installed - skipping diary filing" -ForegroundColor Yellow
    Write-Host "Install from: https://mempalaceofficial.com" -ForegroundColor Yellow
    exit 0
}

# File decision to MemPalace diary
Write-Host "Filing decision to MemPalace diary..." -ForegroundColor Blue

try {
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

    # Build diary entry
    $diaryEntry = "[$timestamp] TaskStatus: $TaskStatus | Decision: $Decision"

    if (-not [string]::IsNullOrWhiteSpace($FilesModified)) {
        $diaryEntry += " | Files: $FilesModified"
    }

    if (-not [string]::IsNullOrWhiteSpace($SessionNotes)) {
        $diaryEntry += " | Notes: $SessionNotes"
    }

    # Write to MemPalace diary
    $result = powershell -NoProfile -Command "mempalace diary-write '$diaryEntry'" 2>&1

    if ($LASTEXITCODE -eq 0) {
        Write-Host "Decision filed successfully:" -ForegroundColor Green
        Write-Host $diaryEntry
        $env:MEMPALACE_DECISION_FILED = "true"
    } else {
        Write-Host "Failed to file decision: $result" -ForegroundColor Red
    }
} catch {
    Write-Host "Error filing decision: $_" -ForegroundColor Red
}

Write-Host "Post-agent hook complete" -ForegroundColor Green