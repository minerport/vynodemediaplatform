param(
    [string]$Configuration = 'Release',
    [string]$Runtime = 'win-x64',
    [string]$UpdatePublicKeyId = $env:WINDOWS_UPDATE_PUBLIC_KEY_ID,
    [string]$UpdatePublicKeySpki = $env:WINDOWS_UPDATE_PUBLIC_KEY_SPKI
)
$ErrorActionPreference = 'Stop'
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$publish = Join-Path $root "artifacts\windows\desktop\$Runtime"
$installer = Join-Path $root 'artifacts\windows\installer'
$publishArguments = @('publish',(Join-Path $root 'apps\windows\VyNode.Windows\VyNode.Windows.csproj'),'-c',$Configuration,'-r',$Runtime,'--self-contained','true','-o',$publish)
if ($UpdatePublicKeyId -and $UpdatePublicKeySpki) {
    $publishArguments += "-p:VyNodeUpdatePublicKeyId=$UpdatePublicKeyId"
    $publishArguments += "-p:VyNodeUpdatePublicKeySpki=$UpdatePublicKeySpki"
}
& dotnet $publishArguments
if ($LASTEXITCODE -ne 0) { throw "Desktop publish failed with exit code $LASTEXITCODE." }
dotnet build (Join-Path $PSScriptRoot 'installer\Desktop\VyNode.Desktop.wixproj') -c $Configuration -p:ClientPublish=$publish -o $installer
if ($LASTEXITCODE -ne 0) { throw "Desktop MSI build failed with exit code $LASTEXITCODE." }
