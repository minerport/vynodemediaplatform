param([string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.ResultRoot 'scm-firewall-acl.json' }

$service = Get-CimInstance Win32_Service -Filter "Name='VyNodeMediaServer'" -ErrorAction SilentlyContinue
$firewall = Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue
$firewallDetails = foreach ($rule in @($firewall)) {
    $port = $rule | Get-NetFirewallPortFilter
    $app = $rule | Get-NetFirewallApplicationFilter
    $address = $rule | Get-NetFirewallAddressFilter
    [ordered]@{ name=$rule.DisplayName; enabled="$($rule.Enabled)"; direction="$($rule.Direction)"; action="$($rule.Action)"; profile="$($rule.Profile)"; protocol="$($port.Protocol)"; localPort="$($port.LocalPort)"; program=$app.Program; remoteAddress=$address.RemoteAddress }
}

$aclSummary = foreach ($path in @($paths.DesktopInstall, $paths.ServerInstall, $paths.ServerData)) {
    if (-not (Test-Path -LiteralPath $path)) { [ordered]@{path=$path; exists=$false}; continue }
    $acl = Get-Acl -LiteralPath $path
    [ordered]@{
        path=$path; exists=$true; owner=$acl.Owner; protected=$acl.AreAccessRulesProtected
        rules=@($acl.Access | ForEach-Object { [ordered]@{identity="$($_.IdentityReference)"; rights="$($_.FileSystemRights)"; type="$($_.AccessControlType)"; inherited=$_.IsInherited} })
    }
}

$quoted = $null
if ($service) { $quoted = $service.PathName -match '^"[^"]+"(?:\s|$)' }
$result = [ordered]@{
    generatedUtc=[DateTime]::UtcNow.ToString('o')
    service=if ($service) { [ordered]@{name=$service.Name; state=$service.State; startMode=$service.StartMode; account=$service.StartName; imagePath=$service.PathName; quotedImagePath=$quoted; processId=$service.ProcessId} } else { $null }
    recovery=(sc.exe qfailure VyNodeMediaServer 2>&1 | Out-String).Trim()
    firewall=@($firewallDetails)
    acl=@($aclSummary)
}
Save-Json $result $OutputPath
$result | ConvertTo-Json -Depth 10

