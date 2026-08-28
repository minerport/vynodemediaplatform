param([switch]$ConfirmMachineChanges)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
Assert-Administrator
if (-not $ConfirmMachineChanges) { throw 'Cleanup requires -ConfirmMachineChanges.' }
$paths = Get-AcceptancePaths

foreach ($msi in @($paths.DesktopMsi, $paths.ServerMsi)) {
    if (Test-Path -LiteralPath $msi) {
        $process = Start-Process msiexec.exe -ArgumentList @('/x',$msi,'/qn','/norestart') -Wait -PassThru
        if ($process.ExitCode -notin @(0,1605)) { throw "Cleanup uninstall failed for $msi with $($process.ExitCode)." }
    }
}

$rule = Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue
if ($rule -and -not (Get-Service VyNodeMediaServer -ErrorAction SilentlyContinue)) {
    $rule | Remove-NetFirewallRule
}

Write-Host 'Exact acceptance packages/rule removed where owned. Persistent server data and media were preserved.'

$feed = Join-Path $paths.AcceptanceRoot 'update-feed'
if (Test-Path -LiteralPath $feed) {
    $resolved = (Resolve-Path -LiteralPath $feed).Path
    $expected = [IO.Path]::GetFullPath($feed)
    if ($resolved -ne $expected -or -not $resolved.StartsWith($paths.AcceptanceRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Refusing unexpected update-feed cleanup target.'
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
    Write-Host 'Removed the exact acceptance-only update feed and its temporary key.'
}
$cleanup = [ordered]@{
    completedUtc=[DateTime]::UtcNow.ToString('o')
    servicePresent=[bool](Get-Service VyNodeMediaServer -ErrorAction SilentlyContinue)
    firewallRuleCount=@(Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue).Count
    persistentServerDataPreserved=(Test-Path -LiteralPath $paths.ServerData)
    updateFeedRemoved=(-not (Test-Path -LiteralPath $feed))
}
Save-Json $cleanup (Join-Path $paths.ResultRoot 'cleanup.json')
& (Join-Path $PSScriptRoot 'New-FinalAcceptanceReport.ps1') | Out-Null
