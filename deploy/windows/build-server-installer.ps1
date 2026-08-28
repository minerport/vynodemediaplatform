param([string]$Configuration = 'Release')
$ErrorActionPreference = 'Stop'
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
& (Join-Path $PSScriptRoot 'build.ps1')
$server = Join-Path $root 'artifacts\windows\vynode-server.exe'
$manager = Join-Path $root 'artifacts\windows\server-manager'
$installer = Join-Path $root 'artifacts\windows\installer'
dotnet publish (Join-Path $root 'apps\windows\VyNode.ServerManager\VyNode.ServerManager.csproj') -c $Configuration -r win-x64 --self-contained true -o $manager
if ($LASTEXITCODE -ne 0) { throw "Server Manager publish failed with exit code $LASTEXITCODE." }
dotnet build (Join-Path $PSScriptRoot 'installer\Server\VyNode.Server.wixproj') -c $Configuration -p:ServerExe=$server -p:ServerManagerPublish=$manager -o $installer
if ($LASTEXITCODE -ne 0) { throw "Server MSI build failed with exit code $LASTEXITCODE." }
