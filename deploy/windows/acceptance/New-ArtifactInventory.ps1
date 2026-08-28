param([string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.AcceptanceRoot 'artifact-inventory.json' }
$items = @(
    @{purpose='Desktop release installer'; path=$paths.DesktopMsi},
    @{purpose='Media Server release installer'; path=$paths.ServerMsi},
    @{purpose='Desktop client'; path=$paths.DesktopExe},
    @{purpose='Media Server'; path=$paths.ServerExe},
    @{purpose='Server Manager'; path=$paths.ManagerExe}
)
$upgradeRoot = Join-Path $paths.AcceptanceRoot 'upgrade-fixtures'
if (Test-Path -LiteralPath $upgradeRoot) {
    Get-ChildItem -LiteralPath $upgradeRoot -Filter *.msi -Recurse | Where-Object { $_.Directory.Name -eq 'packages' } | ForEach-Object {
        $items += @{purpose='Acceptance-only upgrade installer'; path=$_.FullName}
    }
}
$updateClient = Join-Path $paths.AcceptanceRoot 'update-client\packages\VyNode-Desktop-unsigned.msi'
if (Test-Path -LiteralPath $updateClient) {
    $items += @{purpose='Acceptance-only update-enabled Desktop installer'; path=$updateClient}
}
$updateCandidate = Join-Path $paths.AcceptanceRoot 'update-client\candidate-packages\VyNode-Desktop-unsigned.msi'
if (Test-Path -LiteralPath $updateCandidate) {
    $items += @{purpose='Acceptance-only newer update candidate'; path=$updateCandidate}
}
$releaseVersion = Get-MsiProperty $paths.ServerMsi 'ProductVersion'
$inventory = foreach ($item in $items) {
    $version = $null
    if (Test-Path -LiteralPath $item.path) {
        if ([IO.Path]::GetExtension($item.path) -eq '.msi') { $version = Get-MsiProperty $item.path 'ProductVersion' }
        else { $version = (Get-Item -LiteralPath $item.path).VersionInfo.ProductVersion }
    }
    if (-not $version -and $item.purpose -eq 'Media Server') { $version = $releaseVersion }
    [ordered]@{
        filename=[IO.Path]::GetFileName($item.path)
        path=$item.path
        version=$version
        architecture='windows-x64'
        purpose=$item.purpose
        signing=if($item.purpose -like 'Acceptance-only*'){'UNSIGNED ACCEPTANCE-ONLY BUILD'}else{'UNSIGNED DEVELOPMENT BUILD'}
        sha256=(Get-Sha256 $item.path)
        expectedAuthenticodeStatus=if($item.purpose -like 'Acceptance-only*'){'Not intended for publication'}else{'Production release must be Authenticode-signed; current artifact is unsigned'}
    }
}
Save-Json $inventory $OutputPath
$inventory | ConvertTo-Json -Depth 5
