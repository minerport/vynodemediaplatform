. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
$feed = Join-Path $paths.AcceptanceRoot 'update-feed'
if (-not (Test-Path (Join-Path $feed 'manifest.json'))) { throw 'Update acceptance feed has not been generated.' }
$certificate = Get-ChildItem Cert:\CurrentUser\My | Where-Object Subject -eq 'CN=VyNode Update Acceptance' | Select-Object -First 1
if (-not $certificate) {
    $certificate = New-SelfSignedCertificate -Subject 'CN=VyNode Update Acceptance' -TextExtension @('2.5.29.17={text}DNS=localhost&IPAddress=127.0.0.1') -CertStoreLocation Cert:\CurrentUser\My -NotAfter (Get-Date).AddDays(2)
    $public = Join-Path $feed 'acceptance-feed-public.cer'
    Export-Certificate -Cert $certificate -FilePath $public | Out-Null
    Import-Certificate -FilePath $public -CertStoreLocation Cert:\CurrentUser\Root | Out-Null
}
$env:VYNODE_ACCEPTANCE_FEED_ROOT = $feed
$env:VYNODE_ACCEPTANCE_CERT_THUMBPRINT = $certificate.Thumbprint
dotnet run --project (Join-Path $PSScriptRoot 'UpdateFeedServer\UpdateFeedServer.csproj') -c Release --no-launch-profile
