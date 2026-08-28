param([string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.AcceptanceRoot 'preflight.json' }

$artifacts = foreach ($item in @(
    @{ Name='VyNode Desktop MSI'; Path=$paths.DesktopMsi },
    @{ Name='VyNode Media Server MSI'; Path=$paths.ServerMsi },
    @{ Name='VyNode Desktop executable'; Path=$paths.DesktopExe },
    @{ Name='VyNode Media Server executable'; Path=$paths.ServerExe },
    @{ Name='VyNode Server Manager executable'; Path=$paths.ManagerExe }
)) {
    $version = $null
    if (Test-Path -LiteralPath $item.Path) {
        if ([IO.Path]::GetExtension($item.Path) -eq '.msi') { $version = Get-MsiProperty $item.Path 'ProductVersion' }
        else { $version = (Get-Item -LiteralPath $item.Path).VersionInfo.ProductVersion }
    }
    [ordered]@{ name=$item.Name; path=$item.Path; exists=(Test-Path -LiteralPath $item.Path); version=$version; sha256=(Get-Sha256 $item.Path); signing='UNSIGNED DEVELOPMENT BUILD' }
}

$report = [ordered]@{
    generatedUtc = [DateTime]::UtcNow.ToString('o')
    expected = [ordered]@{
        version = '15.0.0'
        serviceName = 'VyNodeMediaServer'
        serviceAccount = 'NT AUTHORITY\LocalService'
        startup = 'Automatic (Delayed Start)'
        port = 8096
        firewallRule = 'VyNode Media Server (Local network)'
        firewallScope = 'TCP 8096, executable-scoped, LocalSubnet'
        programData = $paths.ServerData
        desktopInstall = $paths.DesktopInstall
        serverInstall = $paths.ServerInstall
        serverManager = Join-Path $paths.ServerInstall 'VyNode.ServerManager.exe'
    }
    artifacts = $artifacts
    updateAcceptance = if (Test-Path -LiteralPath (Join-Path $paths.AcceptanceRoot 'update-client\inventory.json')) {
        Get-Content -LiteralPath (Join-Path $paths.AcceptanceRoot 'update-client\inventory.json') -Raw | ConvertFrom-Json
    } else { $null }
}
Save-Json $report $OutputPath
$report | ConvertTo-Json -Depth 10
