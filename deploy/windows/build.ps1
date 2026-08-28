param([string]$Version = '15.0.0', [string]$Commit = 'development')
$ErrorActionPreference = 'Stop'
$output = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\artifacts\windows\vynode-server.exe'))
New-Item -ItemType Directory -Force -Path (Split-Path $output) | Out-Null
if (Get-Command go -ErrorAction SilentlyContinue) {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    go build -trimpath -ldflags "-s -w -X github.com/vynode/media/server/internal/buildinfo.Version=$Version -X github.com/vynode/media/server/internal/buildinfo.Commit=$Commit" -o $output ./server/cmd/vynode-server
} elseif (Get-Command docker -ErrorAction SilentlyContinue) {
    $root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
    docker run --rm -e CGO_ENABLED=0 -e GOOS=windows -e GOARCH=amd64 -v "${root}:/workspace" -w /workspace vynode-server-builder:phase14-5-final go build -trimpath -ldflags "-s -w -X github.com/vynode/media/server/internal/buildinfo.Version=$Version -X github.com/vynode/media/server/internal/buildinfo.Commit=$Commit" -o /workspace/artifacts/windows/vynode-server.exe ./server/cmd/vynode-server
    if ($LASTEXITCODE -ne 0) { throw "Dockerized Windows server build failed with exit code $LASTEXITCODE." }
} else {
    throw 'Go or Docker is required to build the Windows server.'
}
Write-Host "Built $output"
