param([string]$BundleRoot=$PSScriptRoot,[string]$ResultPath=(Join-Path $PSScriptRoot 'client-result.json'))
. (Join-Path $PSScriptRoot 'CleanMachine.Common.ps1')
$result=[ordered]@{environment=[ordered]@{windows=(Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber,OSArchitecture);preflight=(Get-CleanMachinePreflight)};desktop=[ordered]@{};artifacts=[ordered]@{}}
try {
    if(-not $result.environment.preflight.pass){throw 'Developer-tool preflight failed; this is not a literal clean environment.'}
    $msi=Assert-BundleArtifact $BundleRoot 'VyNode-Desktop.msi'
    $result.artifacts.desktopMsiSha256=(Get-FileHash $msi -Algorithm SHA256).Hash
    $log=Join-Path $PSScriptRoot 'desktop-install.log'
    $result.desktop.installExitCode=Invoke-Msi Install $msi $log
    $exe=Join-Path $env:ProgramFiles 'VyNode\Desktop\VyNode.exe'
    $result.desktop.installedExecutable=Test-Path -LiteralPath $exe
    $result.desktop.selfContained=(Test-Path -LiteralPath (Join-Path (Split-Path $exe) 'coreclr.dll'))
    if(-not $result.desktop.installedExecutable -or -not $result.desktop.selfContained){throw 'Installed client is missing or is not self-contained.'}
    Write-Host 'Launch VyNode from the Start menu in the normal user session. Do not run it elevated.'
    $result.desktop.launch=Confirm-Gate 'VyNode launches normally'
    $result.desktop.login=Confirm-Gate 'Global zero-address sign-in succeeds without entering a server address'
    $result.desktop.home=Confirm-Gate 'Home loads from a linked server'
    $result.desktop.search=Confirm-Gate 'Search returns the expected synthetic-media item'
    $result.desktop.movieDetail=Confirm-Gate 'Movie Detail opens'
    $result.desktop.directPlayFullscreen=Confirm-Gate 'Representative Direct Play and fullscreen succeed'
    Get-Process VyNode -ErrorAction SilentlyContinue | Stop-Process
    $uninstall=Join-Path $PSScriptRoot 'desktop-uninstall.log'
    $result.desktop.uninstallExitCode=Invoke-Msi Uninstall $msi $uninstall
    $result.desktop.removed=-not(Test-Path -LiteralPath $exe)
    $result.desktop.pass=($result.desktop.installExitCode -in 0,3010) -and $result.desktop.installedExecutable -and $result.desktop.selfContained -and $result.desktop.launch -and $result.desktop.login -and $result.desktop.home -and $result.desktop.search -and $result.desktop.movieDetail -and $result.desktop.directPlayFullscreen -and $result.desktop.removed
} catch { $result.desktop.pass=$false;$result.desktop.error=$_.Exception.Message;Save-FailureDiagnostics $PSScriptRoot 'desktop' }
Save-CleanResult $result $ResultPath
if(-not $result.desktop.pass){exit 1}
