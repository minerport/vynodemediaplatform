. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
$feed = Join-Path $paths.AcceptanceRoot 'update-feed'
$clientInventoryPath = Join-Path $paths.AcceptanceRoot 'update-client\inventory.json'
foreach ($required in @('manifest.json','manifest.sig','trusted-public-key.spki.base64','VyNode-Desktop-unsigned.msi')) {
    if (-not (Test-Path -LiteralPath (Join-Path $feed $required))) { throw "Missing update fixture file: $required" }
}
if (-not (Test-Path -LiteralPath $clientInventoryPath)) { throw 'Missing update acceptance client inventory.' }
$metadata = [IO.File]::ReadAllBytes((Join-Path $feed 'manifest.json'))
$signature = [IO.File]::ReadAllBytes((Join-Path $feed 'manifest.sig'))
$publicKey = [Convert]::FromBase64String((Get-Content -LiteralPath (Join-Path $feed 'trusted-public-key.spki.base64') -Raw).Trim())
$verifier = [Security.Cryptography.ECDsa]::Create()
try {
    $bytesRead = 0
    $verifier.ImportSubjectPublicKeyInfo($publicKey, [ref]$bytesRead)
    if (-not $verifier.VerifyData($metadata, $signature, [Security.Cryptography.HashAlgorithmName]::SHA256)) {
        throw 'Update fixture metadata signature is invalid.'
    }
} finally { $verifier.Dispose() }
$manifest = [Text.Encoding]::UTF8.GetString($metadata) | ConvertFrom-Json
$package = Join-Path $feed ([IO.Path]::GetFileName(([Uri]$manifest.PackageUrl).AbsolutePath))
if ((Get-Sha256 $package) -ne $manifest.Sha256) { throw 'Update fixture package hash does not match signed metadata.' }
$client = Get-Content -LiteralPath $clientInventoryPath -Raw | ConvertFrom-Json
$result = [ordered]@{
    pass=$true
    acceptanceClientMsi=$client.path
    acceptanceClientSha256=$client.sha256
    trustedKeyId=$client.updateKeyId
    trustedKeyFingerprint=$client.updateKeyFingerprint
    feedPackage=$package
    feedPackageSha256=$manifest.Sha256
    localRuntimeSequence=@('Serve this exact directory over trusted local HTTPS on port 18443','Install the acceptance client with normal UAC','Check for Updates','Verify Ready to Install','Choose Install Update','Approve normal Windows Installer UAC')
}
$result | ConvertTo-Json -Depth 6
