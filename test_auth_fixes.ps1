# Auth Fix Verification Script
# Tests: 1) Protected endpoints accessible after registration
#        2) Token revocation after logout

$baseUrl = "http://localhost:8080"
$timestamp = Get-Date -Format 'yyyyMMddHHmmss'
$email = "testfix-$timestamp@example.com"
$password = "TestPass123!@#"

Write-Host "`n=== AUTH FIX VERIFICATION ===" -ForegroundColor Cyan
Write-Host "Testing: Protected endpoints + Token revocation`n" -ForegroundColor Gray

# 1. REGISTER
Write-Host "1. REGISTER NEW USER" -ForegroundColor Yellow
$regBody = @{
    email = $email
    password = $password
} | ConvertTo-Json

$regResp = curl -s -X POST "$baseUrl/api/v1/auth/register" `
  -H "Content-Type: application/json" `
  -d $regBody

Write-Host "Response: $regResp" -ForegroundColor Gray
$regData = $regResp | ConvertFrom-Json -ErrorAction SilentlyContinue
if ($regData.data.id) {
    Write-Host "✓ 201 Created - User ID: $($regData.data.id)" -ForegroundColor Green
    $userId = $regData.data.id
} else {
    Write-Host "✗ Failed - $($regData.error.message)" -ForegroundColor Red
    exit 1
}

# 2. LOGIN
Write-Host "`n2. LOGIN" -ForegroundColor Yellow
$loginBody = @{
    email = $email
    password = $password
} | ConvertTo-Json

$loginResp = curl -s -X POST "$baseUrl/api/v1/auth/login" `
  -H "Content-Type: application/json" `
  -d $loginBody

Write-Host "Response: $loginResp" -ForegroundColor Gray
$loginData = $loginResp | ConvertFrom-Json -ErrorAction SilentlyContinue
if ($loginData.data.access_token) {
    Write-Host "✓ 200 OK - Login successful" -ForegroundColor Green
    $accessToken = $loginData.data.access_token
    $refreshToken = $loginData.data.refresh_token
    Write-Host "  Access Token: $($accessToken.Substring(0, 30))..." -ForegroundColor Gray
    Write-Host "  Refresh Token: $($refreshToken.Substring(0, 30))..." -ForegroundColor Gray
} else {
    Write-Host "✗ Failed - $($loginData.error.message)" -ForegroundColor Red
    exit 1
}

# 3. TEST PROTECTED ENDPOINT (FIX #1: Should now return 200, was 403)
Write-Host "`n3. ACCESS PROTECTED ENDPOINT (/api/v1/users/me)" -ForegroundColor Yellow
Write-Host "   [FIX #1] Previously returned 403 - should now return 200" -ForegroundColor Cyan

$meResp = curl -s -X GET "$baseUrl/api/v1/users/me" `
  -H "Authorization: Bearer $accessToken"

Write-Host "Response: $meResp" -ForegroundColor Gray
$meData = $meResp | ConvertFrom-Json -ErrorAction SilentlyContinue
if ($meData.data.id) {
    Write-Host "✓ 200 OK - FIX #1 VERIFIED!" -ForegroundColor Green
    Write-Host "  User ID: $($meData.data.id)" -ForegroundColor Gray
    Write-Host "  Email: $($meData.data.email)" -ForegroundColor Gray
} else {
    Write-Host "✗ 403 Forbidden - FIX #1 NOT WORKING" -ForegroundColor Red
    Write-Host "  Error: $($meData.error.message)" -ForegroundColor Red
    exit 1
}

# 4. LOGOUT (Revoke all tokens)
Write-Host "`n4. LOGOUT (REVOKE ALL TOKENS)" -ForegroundColor Yellow
$logoutResp = curl -s -X POST "$baseUrl/api/v1/auth/logout" `
  -H "Authorization: Bearer $accessToken"

Write-Host "Response: $logoutResp" -ForegroundColor Gray
$logoutData = $logoutResp | ConvertFrom-Json -ErrorAction SilentlyContinue
if ($logoutData.data.message) {
    Write-Host "✓ 200 OK - Logout successful" -ForegroundColor Green
    Write-Host "  Message: $($logoutData.data.message)" -ForegroundColor Gray
} else {
    Write-Host "✗ Failed - $($logoutData.error.message)" -ForegroundColor Red
    exit 1
}

# 5. TEST TOKEN REVOCATION (FIX #2: Should return 401, was 200)
Write-Host "`n5. TRY REFRESH AFTER LOGOUT" -ForegroundColor Yellow
Write-Host "   [FIX #2] Should return 401 - token revoked" -ForegroundColor Cyan

$refreshBody = @{
    refresh_token = $refreshToken
} | ConvertTo-Json

$refreshResp = curl -s -X POST "$baseUrl/api/v1/auth/refresh" `
  -H "Content-Type: application/json" `
  -d $refreshBody

Write-Host "Response: $refreshResp" -ForegroundColor Gray
$refreshData = $refreshResp | ConvertFrom-Json -ErrorAction SilentlyContinue
if ($refreshData.error.code -eq "INVALID_REFRESH_TOKEN" -or $refreshData.error.code -eq "TOKEN_REVOKED") {
    Write-Host "✓ 401 Unauthorized - FIX #2 VERIFIED!" -ForegroundColor Green
    Write-Host "  Error Code: $($refreshData.error.code)" -ForegroundColor Gray
    Write-Host "  Message: $($refreshData.error.message)" -ForegroundColor Gray
} else {
    Write-Host "✗ Token not revoked - FIX #2 NOT WORKING" -ForegroundColor Red
    Write-Host "  Error: $($refreshData.error.message)" -ForegroundColor Red
    exit 1
}

# SUMMARY
Write-Host "`n=== VERIFICATION SUMMARY ===" -ForegroundColor Cyan
Write-Host "✓ FIX #1: Protected endpoints now accessible (200 OK)" -ForegroundColor Green
Write-Host "✓ FIX #2: Token revocation working (401 on refresh)" -ForegroundColor Green
Write-Host "`n✓ ALL TESTS PASSED - Auth fixes verified!" -ForegroundColor Green
