param([Parameter(Mandatory)][ValidateSet('Before','After')] [string]$Phase, [string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.ResultRoot "client-upgrade-$($Phase.ToLowerInvariant()).json" }
$statePath = Join-Path $env:LOCALAPPDATA 'VyNode\Desktop\session.json'
$state = $null
if (Test-Path -LiteralPath $statePath) { $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json }
$result = [ordered]@{
    phase=$Phase; capturedUtc=[DateTime]::UtcNow.ToString('o'); stateFilePresent=(Test-Path -LiteralPath $statePath)
    globalSessionPresent=[bool]($state.globalAccountId -or $state.globalUserName)
    serverProfileCount=@($state.servers).Count
    selectedServerId=$state.selectedServerId
    secureCredentialValuesInspected=$false
}
Save-Json $result $OutputPath
$result | ConvertTo-Json
