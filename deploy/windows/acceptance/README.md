# Phase 15 elevated acceptance kit

These scripts prepare and run the final Windows platform acceptance checks. They
do not bypass UAC. The elevated orchestrator exits before changing the machine
unless it is already running as Administrator.

Remote-safe preparation:

    pwsh ./deploy/windows/acceptance/New-PreflightReport.ps1
    pwsh ./deploy/windows/acceptance/New-UpgradeFixtures.ps1
    pwsh ./deploy/windows/acceptance/New-UpdateRuntimeFixture.ps1
    pwsh ./deploy/windows/acceptance/New-UpdateAcceptanceClient.ps1

When an interactive user is present, open one elevated PowerShell and run:

    pwsh ./deploy/windows/acceptance/Invoke-Phase15ElevatedAcceptance.ps1 -Stage Preflight

The orchestrator is staged and requires an explicit ConfirmMachineChanges switch
for installation, service, failure, or uninstall stages. It never approves its
own elevation. Results are written below artifacts/windows/acceptance/results;
secrets and private identity material are never collected.

After the requested reboot:

    pwsh ./deploy/windows/acceptance/Test-PostReboot.ps1

Cleanup only targets exact acceptance packages and the exact installer firewall
rule. It never deletes ProgramData server state or user media.

The update acceptance client is a Release build pinned only to the ignored
acceptance public key and local HTTPS feed. Its matching private key remains below
ignored artifacts and is removed by cleanup. Production builds receive only the
official public key through CI build variables.

Run Test-UpdateAcceptanceFixture.ps1 before the local session. It verifies the
detached signature, package hash, pinned public-key fingerprint, and exact
acceptance-client MSI without launching anything. The later HTTPS endpoint must use
a certificate trusted by the Windows test profile; VyNode never disables TLS
validation for an acceptance feed.
