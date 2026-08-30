param(
    [Parameter(Mandatory)][ValidateSet('stable','beta')][string]$Channel,
    [Parameter(Mandatory)][string]$Version,
    [Parameter(Mandatory)][Uri]$PackageUrl,
    [string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
foreach ($name in 'WINDOWS_UPDATE_PUBLIC_KEY_ID','WINDOWS_UPDATE_SIGNING_PRIVATE_KEY_BASE64') {
    if (-not [Environment]::GetEnvironmentVariable($name)) { throw "Production update metadata requires $name." }
}
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root 'artifacts\windows\release\update' }
$package = Join-Path $root 'artifacts\windows\installer\VyNode-Desktop.msi'
if ((Get-AuthenticodeSignature -LiteralPath $package).Status -ne 'Valid') { throw 'Update metadata requires the final Authenticode-valid Desktop MSI.' }
$manifest = [ordered]@{
    Channel=$Channel
    Version=$Version
    PackageUrl=$PackageUrl.AbsoluteUri
    Sha256=(Get-FileHash -LiteralPath $package -Algorithm SHA256).Hash
    MinimumClientVersion='16.0.0'
    PublishedAt=[DateTime]::UtcNow.ToString('o')
    SigningKeyId=$env:WINDOWS_UPDATE_PUBLIC_KEY_ID
}
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
$metadata = [Text.Encoding]::UTF8.GetBytes(($manifest | ConvertTo-Json -Compress))
[IO.File]::WriteAllBytes((Join-Path $OutputDirectory 'manifest.json'),$metadata)
$key = [Security.Cryptography.ECDsa]::Create()
try {
    $privateBytes=[Convert]::FromBase64String($env:WINDOWS_UPDATE_SIGNING_PRIVATE_KEY_BASE64)
    $read=0
    $key.ImportPkcs8PrivateKey($privateBytes,[ref]$read)
    [IO.File]::WriteAllBytes((Join-Path $OutputDirectory 'manifest.sig'),$key.SignData($metadata,[Security.Cryptography.HashAlgorithmName]::SHA256))
} finally { $key.Dispose(); if($privateBytes){[Array]::Clear($privateBytes,0,$privateBytes.Length)} }
Write-Host "Created signed $Channel update metadata for the final signed MSI hash $($manifest.Sha256)."
