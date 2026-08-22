$ErrorActionPreference = 'Stop'
$output = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\artifacts\windows\vynode-server.exe'))
New-Item -ItemType Directory -Force -Path (Split-Path $output) | Out-Null
$env:CGO_ENABLED = '0'
go build -trimpath -o $output ./server/cmd/vynode-server
Write-Host "Built $output"

