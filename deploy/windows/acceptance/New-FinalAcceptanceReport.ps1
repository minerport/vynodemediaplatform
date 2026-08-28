param([string]$OutputPath)
. (Join-Path $PSScriptRoot 'Acceptance.Common.ps1')
$paths = Get-AcceptancePaths
if (-not $OutputPath) { $OutputPath = Join-Path $paths.ResultRoot 'phase-15-elevated-acceptance.json' }

function Read-Result([string]$Name) {
    $path = Join-Path $paths.ResultRoot $Name
    if (-not (Test-Path -LiteralPath $path)) { return $null }
    Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
}

$preflightPath = Join-Path $paths.AcceptanceRoot 'preflight.json'
$inventoryPath = Join-Path $paths.AcceptanceRoot 'artifact-inventory.json'
$preflight = if(Test-Path $preflightPath){Get-Content $preflightPath -Raw|ConvertFrom-Json}else{$null}
$inventory = if(Test-Path $inventoryPath){Get-Content $inventoryPath -Raw|ConvertFrom-Json}else{@()}
$machine = Read-Result 'scm-firewall-acl.json'
$installed = Read-Result 'installed-products.json'
$serverBefore = Read-Result 'server-upgrade-before.json'
$serverAfter = Read-Result 'server-upgrade-after.json'
$clientBefore = Read-Result 'client-upgrade-before.json'
$clientAfter = Read-Result 'client-upgrade-after.json'
$postReboot = Read-Result 'post-reboot.json'
$cleanup = Read-Result 'cleanup.json'
$updateFixture = Join-Path $paths.AcceptanceRoot 'update-client\inventory.json'

$report = [ordered]@{
    schemaVersion=1
    generatedUtc=[DateTime]::UtcNow.ToString('o')
    overall=if($machine -and $serverAfter -and $clientAfter -and $postReboot -and $cleanup){'REVIEW_REQUIRED'}else{'PENDING_INTERACTIVE_ELEVATION'}
    clientMsi=@($inventory|Where-Object purpose -eq 'Desktop release installer')
    serverMsi=@($inventory|Where-Object purpose -eq 'Media Server release installer')
    installedProducts=$installed
    scmService=$machine.service
    serviceRecovery=$machine.recovery
    serviceAccountAndImagePath=if($machine){[ordered]@{account=$machine.service.account;imagePath=$machine.service.imagePath;quoted=$machine.service.quotedImagePath}}else{$null}
    acl=$machine.acl
    firewall=$machine.firewall
    serverIdentityPreservation=[ordered]@{before=$serverBefore;after=$serverAfter}
    upgrades=[ordered]@{server=[ordered]@{before=$serverBefore;after=$serverAfter};client=[ordered]@{before=$clientBefore;after=$clientAfter}}
    uninstallReinstall=[ordered]@{installedState=$installed;cleanup=$cleanup}
    rebootStartup=$postReboot
    updateHandoff=if(Test-Path $updateFixture){Get-Content $updateFixture -Raw|ConvertFrom-Json}else{$null}
    artifacts=$inventory
    cleanup=$cleanup
    expected=$preflight.expected
}
Save-Json $report $OutputPath
$report | ConvertTo-Json -Depth 12
