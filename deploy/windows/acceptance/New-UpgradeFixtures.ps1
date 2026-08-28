param(
    [string]$VersionN = '15.0.0',
    [string]$VersionNPlusOne = '15.0.1'
)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
$fixtureRoot = Join-Path $paths.AcceptanceRoot 'upgrade-fixtures'
$sourceDesktop = Join-Path $paths.Root 'deploy\windows\installer\Desktop'
$sourceServer = Join-Path $paths.Root 'deploy\windows\installer\Server'

function Copy-InstallerSource([string]$Source, [string]$Target) {
    New-Item -ItemType Directory -Path $Target -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $Source 'Package.wxs') -Destination $Target
    Copy-Item -LiteralPath (Get-ChildItem -LiteralPath $Source -Filter *.wixproj | Select-Object -First 1).FullName -Destination $Target
}

function Set-PackageVersion([string]$PackagePath, [string]$Version) {
    [xml]$xml = Get-Content -LiteralPath $PackagePath -Raw
    $xml.Wix.Package.Version = $Version
    $xml.Save($PackagePath)
}

function Build-Fixture([string]$Version) {
    $target = Join-Path $fixtureRoot $Version
    if (Test-Path -LiteralPath $target) {
        throw "Acceptance fixture directory already exists: $target. Remove that exact generated directory before rebuilding."
    }
    $desktop = Join-Path $target 'Desktop'
    $server = Join-Path $target 'Server'
    Copy-InstallerSource $sourceDesktop $desktop
    Copy-InstallerSource $sourceServer $server
    Set-PackageVersion (Join-Path $desktop 'Package.wxs') $Version
    Set-PackageVersion (Join-Path $server 'Package.wxs') $Version
    $output = Join-Path $target 'packages'
    New-Item -ItemType Directory -Path $output -Force | Out-Null

    $desktopPublish = Split-Path -Parent $paths.DesktopExe
    $managerPublish = Split-Path -Parent $paths.ManagerExe
    & dotnet @('build',(Join-Path $desktop 'VyNode.Desktop.wixproj'),'-c','Release',"-p:ClientPublish=$desktopPublish",'-o',$output) | Write-Host
    if ($LASTEXITCODE -ne 0) { throw "Desktop acceptance MSI $Version failed." }
    & dotnet @('build',(Join-Path $server 'VyNode.Server.wixproj'),'-c','Release',"-p:ServerExe=$($paths.ServerExe)","-p:ServerManagerPublish=$managerPublish",'-o',$output) | Write-Host
    if ($LASTEXITCODE -ne 0) { throw "Server acceptance MSI $Version failed." }

    $inventory = Get-ChildItem -LiteralPath $output -Filter *.msi | ForEach-Object {
        [ordered]@{path=$_.FullName; version=(Get-MsiProperty $_.FullName 'ProductVersion'); sha256=(Get-Sha256 $_.FullName); signing='UNSIGNED ACCEPTANCE-ONLY BUILD'}
    }
    Save-Json $inventory (Join-Path $target 'inventory.json')
    $inventory
}

$all = @()
$all += Build-Fixture $VersionN
$all += Build-Fixture $VersionNPlusOne
Save-Json $all (Join-Path $fixtureRoot 'inventory.json')
$all | ConvertTo-Json -Depth 5
