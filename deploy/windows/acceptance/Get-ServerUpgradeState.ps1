param([Parameter(Mandatory)][ValidateSet('Before','After')] [string]$Phase, [string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.ResultRoot "server-upgrade-$($Phase.ToLowerInvariant()).json" }
$db = Join-Path $paths.ServerData 'vynode.db'

$connection = $null
try { $connection = Invoke-RestMethod -Uri 'http://127.0.0.1:8096/api/v1/connection-info' -TimeoutSec 5 } catch {}
$version = $null
try { $version = Invoke-RestMethod -Uri 'http://127.0.0.1:8096/api/v1/system/version' -TimeoutSec 5 } catch {}
$identityPath = Join-Path $paths.ServerData 'connect\server-identity.key'
$privateIdentity = Test-Path -LiteralPath $identityPath
$identityFingerprint = if ($privateIdentity) { Get-Sha256 $identityPath } else { $null }
$databaseHash = $null
if (Test-Path -LiteralPath $db) {
    try { $databaseHash = Get-Sha256 $db } catch { $databaseHash = $null }
}
$state = [ordered]@{
    phase=$Phase; capturedUtc=[DateTime]::UtcNow.ToString('o'); ready=$false; version=$version
    installationId=$connection.serverId
    publicKeyFingerprint=$identityFingerprint
    database=[ordered]@{exists=(Test-Path -LiteralPath $db); length=if(Test-Path -LiteralPath $db){(Get-Item $db).Length}else{$null}; sha256=$databaseHash}
    configFiles=@(Get-ChildItem -LiteralPath $paths.ServerData -File -ErrorAction SilentlyContinue | Where-Object Name -NotMatch '(?i)key|token|credential|secret' | Select-Object -ExpandProperty Name)
    privateIdentityPresent=$privateIdentity
}
try { Invoke-WebRequest -Uri 'http://127.0.0.1:8096/ready' -TimeoutSec 5 -UseBasicParsing | Out-Null; $state.ready=$true } catch {
    try { Invoke-WebRequest -Uri 'http://127.0.0.1:8096/health' -TimeoutSec 5 -UseBasicParsing | Out-Null; $state.ready=$true } catch {}
}
Save-Json $state $OutputPath
$state | ConvertTo-Json -Depth 10
