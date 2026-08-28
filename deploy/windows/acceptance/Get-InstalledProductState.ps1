param([string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.ResultRoot 'installed-products.json' }
$uninstallRoots = @(
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
$products = Get-ItemProperty $uninstallRoots -ErrorAction SilentlyContinue | Where-Object DisplayName -like 'VyNode*' | Select-Object DisplayName,DisplayVersion,InstallLocation,UninstallString
$state = [ordered]@{
    generatedUtc=[DateTime]::UtcNow.ToString('o')
    products=@($products)
    desktop=[ordered]@{installed=(Test-Path (Join-Path $paths.DesktopInstall 'VyNode.exe')); startMenu=(Test-Path (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\VyNode\VyNode.lnk'))}
    server=[ordered]@{installed=(Test-Path (Join-Path $paths.ServerInstall 'vynode-server.exe')); manager=(Test-Path (Join-Path $paths.ServerInstall 'VyNode.ServerManager.exe')); servicePresent=[bool](Get-Service VyNodeMediaServer -ErrorAction SilentlyContinue)}
    serverFirewallRuleCount=@(Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue).Count
}
Save-Json $state $OutputPath
$state | ConvertTo-Json -Depth 8

