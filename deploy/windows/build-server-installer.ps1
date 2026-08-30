param([string]$Configuration = 'Release', [string]$Version = '16.0.0', [string]$MediaToolsManifest, [string]$MediaToolsArchive)
$ErrorActionPreference = 'Stop'
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
& (Join-Path $PSScriptRoot 'build.ps1') -Version $Version
$server = Join-Path $root 'artifacts\windows\vynode-server.exe'
$manager = Join-Path $root 'artifacts\windows\server-manager'
$installer = Join-Path $root 'artifacts\windows\installer'
$web = Join-Path $root 'apps\web\dist'
Push-Location $root
try {
    npm run build --workspace @vynode/web
    if ($LASTEXITCODE -ne 0) { throw 'Web Admin build failed.' }
} finally { Pop-Location }
if (-not (Test-Path -LiteralPath (Join-Path $web 'index.html'))) { throw 'Web Admin entry point is missing.' }
$mediaTools = Join-Path $root 'artifacts\windows\media-tools\payload'
if ($MediaToolsManifest) { $mediaTools = Join-Path $root ('artifacts\windows\media-tools\payload-' + $Version) }
& (Join-Path $PSScriptRoot 'media-tools\Get-MediaTools.ps1') -Destination $mediaTools -UseCache -DefinitionPath $MediaToolsManifest -ArchivePath $MediaToolsArchive
dotnet publish (Join-Path $root 'apps\windows\VyNode.ServerManager\VyNode.ServerManager.csproj') -c $Configuration -r win-x64 --self-contained true -o $manager
if ($LASTEXITCODE -ne 0) { throw "Server Manager publish failed with exit code $LASTEXITCODE." }
dotnet build (Join-Path $PSScriptRoot 'installer\Server\VyNode.Server.wixproj') -c $Configuration -p:PackageVersion=$Version -p:ServerExe=$server -p:ServerManagerPublish=$manager -p:MediaToolsPayload=$mediaTools -p:WebPayload=$web -o $installer
if ($LASTEXITCODE -ne 0) { throw "Server MSI build failed with exit code $LASTEXITCODE." }
