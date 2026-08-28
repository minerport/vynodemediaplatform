param([int]$ReadyTimeoutSeconds = 120)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$deadline = [DateTime]::UtcNow.AddSeconds($ReadyTimeoutSeconds)
do {
    $service = Get-Service VyNodeMediaServer -ErrorAction SilentlyContinue
    $ready = $false
    try { Invoke-WebRequest -Uri 'http://127.0.0.1:8096/ready' -UseBasicParsing -TimeoutSec 3 | Out-Null; $ready=$true } catch {
        try { Invoke-WebRequest -Uri 'http://127.0.0.1:8096/health' -UseBasicParsing -TimeoutSec 3 | Out-Null; $ready=$true } catch {}
    }
    if ($service.Status -eq 'Running' -and $ready) { break }
    Start-Sleep -Seconds 2
} while ([DateTime]::UtcNow -lt $deadline)
$result = [ordered]@{serviceStatus="$($service.Status)"; ready=$ready; startupPass=($service.Status -eq 'Running' -and $ready); checkedUtc=[DateTime]::UtcNow.ToString('o')}
$paths=Get-AcceptancePaths
Save-Json $result (Join-Path $paths.ResultRoot 'post-reboot.json')
$result | ConvertTo-Json
if (-not $result.startupPass) { exit 1 }

