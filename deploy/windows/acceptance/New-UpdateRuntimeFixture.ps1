param(
    [string]$CandidateVersion = '15.0.1',
    [string]$PackagePath,
    [switch]$ReuseExistingKey
)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
$feedRoot = Join-Path $paths.AcceptanceRoot 'update-feed'
if (-not $PackagePath) { $PackagePath = Join-Path $paths.AcceptanceRoot 'update-client\candidate-packages\VyNode-Desktop-unsigned.msi' }
if (-not (Test-Path -LiteralPath $PackagePath)) { throw "Candidate package not found: $PackagePath" }
if ((Test-Path -LiteralPath $feedRoot) -and -not $ReuseExistingKey) { throw "Fixture already exists: $feedRoot. Use ReuseExistingKey only to refresh it with the retained acceptance key." }
New-Item -ItemType Directory -Path $feedRoot -Force | Out-Null

$trusted = [Security.Cryptography.ECDsa]::Create()
$curve = [Security.Cryptography.ECCurve]::CreateFromFriendlyName('nistP256')
if ($ReuseExistingKey) {
    $privateBytes = [IO.File]::ReadAllBytes((Join-Path $feedRoot 'acceptance-private-key.pkcs8'))
    $bytesRead = 0
    $trusted.ImportPkcs8PrivateKey($privateBytes, [ref]$bytesRead)
} else { $trusted.GenerateKey($curve) }
$wrong = [Security.Cryptography.ECDsa]::Create()
$wrong.GenerateKey($curve)
try {
    $packageName = [IO.Path]::GetFileName($PackagePath)
    Copy-Item -LiteralPath $PackagePath -Destination (Join-Path $feedRoot $packageName) -Force
    $hash = Get-Sha256 (Join-Path $feedRoot $packageName)
    $manifest = [ordered]@{
        Channel='stable'
        Version=$CandidateVersion
        PackageUrl="https://127.0.0.1:18443/$packageName"
        Sha256=$hash
        MinimumClientVersion='15.0.0'
        PublishedAt=[DateTime]::UtcNow.ToString('o')
        SigningKeyId='acceptance-2026'
    }
    $metadata = [Text.Encoding]::UTF8.GetBytes(($manifest | ConvertTo-Json -Compress))
    [IO.File]::WriteAllBytes((Join-Path $feedRoot 'manifest.json'), $metadata)
    [IO.File]::WriteAllBytes((Join-Path $feedRoot 'manifest.sig'), $trusted.SignData($metadata, [Security.Cryptography.HashAlgorithmName]::SHA256))
    [IO.File]::WriteAllBytes((Join-Path $feedRoot 'manifest-wrong-key.sig'), $wrong.SignData($metadata, [Security.Cryptography.HashAlgorithmName]::SHA256))
    $tampered = [byte[]]$metadata.Clone()
    $tampered[$tampered.Length - 2] = $tampered[$tampered.Length - 2] -bxor 1
    [IO.File]::WriteAllBytes((Join-Path $feedRoot 'manifest-tampered.json'), $tampered)
    Copy-Item -LiteralPath (Join-Path $feedRoot $packageName) -Destination (Join-Path $feedRoot 'package-tampered.msi') -Force
    Add-Content -LiteralPath (Join-Path $feedRoot 'package-tampered.msi') -Value 'tampered' -NoNewline
    [IO.File]::WriteAllText((Join-Path $feedRoot 'trusted-public-key.spki.base64'), [Convert]::ToBase64String($trusted.ExportSubjectPublicKeyInfo()))
    [IO.File]::WriteAllBytes((Join-Path $feedRoot 'acceptance-private-key.pkcs8'), $trusted.ExportPkcs8PrivateKey())
    [IO.File]::WriteAllText((Join-Path $feedRoot 'README.txt'), 'ACCEPTANCE-ONLY. Private key and feed are ignored artifacts. Delete the entire exact update-feed directory after runtime acceptance.')
} finally {
    $trusted.Dispose()
    $wrong.Dispose()
}

[ordered]@{
    path=$feedRoot
    package=$packageName
    packageSha256=$hash
    privateKeyTracked=$false
    runtimeBoundary='Verification and installer handoff are wired; interactive UAC runtime acceptance remains pending.'
} | ConvertTo-Json
