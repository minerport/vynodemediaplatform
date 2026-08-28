param([switch]$InventoryOnly)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
$feed = Join-Path $paths.AcceptanceRoot 'update-feed'
$publicKeyPath = Join-Path $feed 'trusted-public-key.spki.base64'
if (-not (Test-Path -LiteralPath $publicKeyPath)) { throw 'Run New-UpdateRuntimeFixture.ps1 first.' }
$publicKey = (Get-Content -LiteralPath $publicKeyPath -Raw).Trim()
$root = Join-Path $paths.AcceptanceRoot 'update-client'
if ((Test-Path -LiteralPath $root) -and -not $InventoryOnly) { throw "Acceptance update client already exists: $root" }
$publish = Join-Path $root 'publish'
$clientSource = Join-Path $root 'source'
$installerSource = Join-Path $root 'installer'
$packages = Join-Path $root 'packages'
$candidateInstaller = Join-Path $root 'candidate-installer'
$candidatePackages = Join-Path $root 'candidate-packages'
if (-not $InventoryOnly) {
New-Item -ItemType Directory -Path $publish,$clientSource,$installerSource,$packages,$candidateInstaller,$candidatePackages -Force | Out-Null
$productionSource = Join-Path $paths.Root 'apps\windows\VyNode.Windows'
Get-ChildItem -LiteralPath $productionSource | Where-Object Name -NotIn @('bin','obj') | ForEach-Object {
    Copy-Item -LiteralPath $_.FullName -Destination $clientSource -Recurse
}

& dotnet @(
    'publish',(Join-Path $clientSource 'VyNode.Windows.csproj'),
    '-c','Release','-r','win-x64','--self-contained','true','-o',$publish,
    '-p:VyNodeUpdatePublicKeyId=acceptance-2026',
    "-p:VyNodeUpdatePublicKeySpki=$publicKey",
    '-p:VyNodeUpdateMetadataUrl=https://127.0.0.1:18443/manifest.json',
    '-p:VyNodeUpdateSignatureUrl=https://127.0.0.1:18443/manifest.sig'
) | Write-Host
if ($LASTEXITCODE -ne 0) { throw 'Acceptance update client publish failed.' }

$source = Join-Path $paths.Root 'deploy\windows\installer\Desktop'
Copy-Item -LiteralPath (Join-Path $source 'Package.wxs') -Destination $installerSource
Copy-Item -LiteralPath (Join-Path $source 'VyNode.Desktop.wixproj') -Destination $installerSource
& dotnet @('build',(Join-Path $installerSource 'VyNode.Desktop.wixproj'),'-c','Release',"-p:ClientPublish=$publish",'-o',$packages) | Write-Host
if ($LASTEXITCODE -ne 0) { throw 'Acceptance update client MSI build failed.' }

Copy-Item -LiteralPath (Join-Path $source 'Package.wxs') -Destination $candidateInstaller
Copy-Item -LiteralPath (Join-Path $source 'VyNode.Desktop.wixproj') -Destination $candidateInstaller
[xml]$candidateXml = Get-Content -LiteralPath (Join-Path $candidateInstaller 'Package.wxs') -Raw
$candidateXml.Wix.Package.Version = '15.0.1'
$candidateXml.Save((Join-Path $candidateInstaller 'Package.wxs'))
& dotnet @('build',(Join-Path $candidateInstaller 'VyNode.Desktop.wixproj'),'-c','Release',"-p:ClientPublish=$publish",'-o',$candidatePackages) | Write-Host
if ($LASTEXITCODE -ne 0) { throw 'Acceptance update candidate MSI build failed.' }
}

$msi = Join-Path $packages 'VyNode-Desktop-unsigned.msi'
$candidateMsi = Join-Path $candidatePackages 'VyNode-Desktop-unsigned.msi'
$inventory = [ordered]@{
    path=$msi
    version=(Get-MsiProperty $msi 'ProductVersion')
    updateKeyId='acceptance-2026'
    updateKeyFingerprint=([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Convert]::FromBase64String($publicKey))))
    expectedFeed='https://127.0.0.1:18443'
    sha256=(Get-Sha256 $msi)
    candidatePath=$candidateMsi
    candidateVersion=(Get-MsiProperty $candidateMsi 'ProductVersion')
    candidateSha256=(Get-Sha256 $candidateMsi)
    signing='UNSIGNED ACCEPTANCE-ONLY BUILD'
}
Save-Json $inventory (Join-Path $root 'inventory.json')
$inventory | ConvertTo-Json
