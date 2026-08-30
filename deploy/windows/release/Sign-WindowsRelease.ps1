param([string]$TimestampUrl = $env:WINDOWS_SIGNING_TIMESTAMP_URL, [string]$Version = '16.0.0')
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
function Resolve-SignTool {
    $command=Get-Command signtool.exe -ErrorAction SilentlyContinue
    if($command){return $command.Source}
    $kits=Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    $candidate=Get-ChildItem $kits -Filter signtool.exe -File -Recurse -ErrorAction SilentlyContinue | Where-Object FullName -match '\\x64\\signtool\.exe$' | Sort-Object FullName -Descending | Select-Object -First 1
    if(-not $candidate){
        $nuget=Join-Path $env:USERPROFILE '.nuget\packages\microsoft.windows.sdk.buildtools'
        $candidate=Get-ChildItem $nuget -Filter signtool.exe -File -Recurse -ErrorAction SilentlyContinue | Where-Object FullName -match '\\x64\\signtool\.exe$' | Sort-Object FullName -Descending | Select-Object -First 1
    }
    if(-not $candidate){throw 'Windows SDK signtool.exe was not found.'}
    $candidate.FullName
}
$signtool=Resolve-SignTool
foreach ($name in 'WINDOWS_SIGNING_CERT_BASE64','WINDOWS_SIGNING_CERT_PASSWORD','WINDOWS_SIGNING_EXPECTED_SUBJECT','WINDOWS_SIGNING_EXPECTED_THUMBPRINT','WINDOWS_UPDATE_PUBLIC_KEY_ID','WINDOWS_UPDATE_PUBLIC_KEY_SPKI') {
    if (-not [Environment]::GetEnvironmentVariable($name)) { throw "Production signing requires $name." }
}
if (-not $TimestampUrl) { throw 'Production signing requires WINDOWS_SIGNING_TIMESTAMP_URL or -TimestampUrl.' }
$certificate = Join-Path ([IO.Path]::GetTempPath()) "vynode-signing-$([guid]::NewGuid().ToString('N')).pfx"
try {
    [IO.File]::WriteAllBytes($certificate, [Convert]::FromBase64String($env:WINDOWS_SIGNING_CERT_BASE64))
    $executables = @(
        (Join-Path $root 'artifacts\windows\desktop\win-x64\VyNode.exe'),
        (Join-Path $root 'artifacts\windows\vynode-server.exe'),
        (Join-Path $root 'artifacts\windows\server-manager\VyNode.ServerManager.exe')
    )
    foreach ($target in $executables) {
        & $signtool sign /fd SHA256 /td SHA256 /tr $TimestampUrl /f $certificate /p $env:WINDOWS_SIGNING_CERT_PASSWORD $target
        if ($LASTEXITCODE -ne 0) { throw "Signing failed for $target." }
        & $signtool verify /pa /v $target
        if ($LASTEXITCODE -ne 0) { throw "Signature verification failed for $target." }
    }
    $installer = Join-Path $root 'artifacts\windows\installer'
    $mediaTools = Join-Path $root 'artifacts\windows\media-tools\payload'
    dotnet build (Join-Path $root 'deploy\windows\installer\Desktop\VyNode.Desktop.wixproj') -c Release -p:PackageVersion=$Version -p:ClientPublish=(Join-Path $root 'artifacts\windows\desktop\win-x64') -o $installer
    if ($LASTEXITCODE -ne 0) { throw 'Signed Desktop MSI rebuild failed.' }
    dotnet build (Join-Path $root 'deploy\windows\installer\Server\VyNode.Server.wixproj') -c Release -p:PackageVersion=$Version -p:ServerExe=(Join-Path $root 'artifacts\windows\vynode-server.exe') -p:ServerManagerPublish=(Join-Path $root 'artifacts\windows\server-manager') -p:MediaToolsPayload=$mediaTools -o $installer
    if ($LASTEXITCODE -ne 0) { throw 'Signed Server MSI rebuild failed.' }
    foreach ($target in @((Join-Path $installer 'VyNode-Desktop-unsigned.msi'),(Join-Path $installer 'VyNode-Media-Server-unsigned.msi'))) {
        if(-not(Test-Path -LiteralPath $target -PathType Leaf)){throw "Expected unsigned MSI build output is missing: $target"}
        & $signtool sign /fd SHA256 /td SHA256 /tr $TimestampUrl /f $certificate /p $env:WINDOWS_SIGNING_CERT_PASSWORD $target
        if ($LASTEXITCODE -ne 0) { throw "Signing failed for $target." }
        & $signtool verify /pa /v $target
        if ($LASTEXITCODE -ne 0) { throw "Signature verification failed for $target." }
    }
    Move-Item -LiteralPath (Join-Path $installer 'VyNode-Desktop-unsigned.msi') -Destination (Join-Path $installer 'VyNode-Desktop.msi') -Force
    Move-Item -LiteralPath (Join-Path $installer 'VyNode-Media-Server-unsigned.msi') -Destination (Join-Path $installer 'VyNode-Media-Server.msi') -Force
    & (Join-Path $PSScriptRoot 'Test-AuthenticodeRelease.ps1')
    if ($LASTEXITCODE -ne 0) { throw 'Detailed Authenticode verification failed.' }
} finally {
    Remove-Item -LiteralPath $certificate -Force -ErrorAction SilentlyContinue
}
