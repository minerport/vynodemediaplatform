$ErrorActionPreference = 'Stop'

function Get-RepositoryRoot {
    [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Assert-Administrator {
    if (-not (Test-IsAdministrator)) {
        throw 'BLOCKED - interactive UAC approval required. Reopen PowerShell as Administrator; this script will not bypass UAC.'
    }
}

function Get-AcceptancePaths {
    $root = Get-RepositoryRoot
    [ordered]@{
        Root = $root
        AcceptanceRoot = Join-Path $root 'artifacts\windows\acceptance'
        ResultRoot = Join-Path $root 'artifacts\windows\acceptance\results'
        DesktopMsi = Join-Path $root 'artifacts\windows\installer\VyNode-Desktop-unsigned.msi'
        ServerMsi = Join-Path $root 'artifacts\windows\installer\VyNode-Media-Server-unsigned.msi'
        DesktopExe = Join-Path $root 'artifacts\windows\desktop\win-x64\VyNode.exe'
        ServerExe = Join-Path $root 'artifacts\windows\vynode-server.exe'
        ManagerExe = Join-Path $root 'artifacts\windows\server-manager\VyNode.ServerManager.exe'
        DesktopInstall = Join-Path $env:ProgramFiles 'VyNode\Desktop'
        ServerInstall = Join-Path $env:ProgramFiles 'VyNode\Media Server'
        ServerData = Join-Path $env:ProgramData 'VyNode\Media Server'
    }
}

function Get-Sha256([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
}

function Save-Json([object]$Value, [string]$Path) {
    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    $Value | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $Path -Encoding utf8
}

function Get-MsiProperty([string]$Path, [string]$Name) {
    $installer = New-Object -ComObject WindowsInstaller.Installer
    $database = $installer.GetType().InvokeMember('OpenDatabase', 'InvokeMethod', $null, $installer, @($Path, 0))
    $sql = "SELECT Value FROM Property WHERE Property='$Name'"
    $view = $database.GetType().InvokeMember('OpenView', 'InvokeMethod', $null, $database, @($sql))
    $view.GetType().InvokeMember('Execute', 'InvokeMethod', $null, $view, $null) | Out-Null
    $record = $view.GetType().InvokeMember('Fetch', 'InvokeMethod', $null, $view, $null)
    if ($null -eq $record) { return $null }
    $record.GetType().InvokeMember('StringData', 'GetProperty', $null, $record, 1)
}

