# VyNode clean-Windows acceptance bundle

For the real gate, generate this directory with `New-CleanMachineBundle.ps1
-Production`; unsigned preparation bundles are not acceptance evidence. Copy only
the production generated bundle to a fresh disposable Windows 11 x64 VM. The VM
must have no repository checkout, Visual Studio, .NET SDK, Go, Node/npm, WiX,
Android tooling, FFmpeg/FFprobe, development certificates, or VyNode development
environment variables. Preserve logs and result JSON only inside the ignored
acceptance evidence location; never add credentials.

Run `Test-CleanClient.ps1` from a normal PowerShell session. UAC is requested only
for MSI operations. The operator performs login and visual playback checks without
entering credentials into the script.

For the server gate, transfer the bundle, disconnect Internet or isolate the VM
network, and run `Test-CleanServer.ps1 -OfflineIsolationConfirmed`. Use the included
synthetic MP4 as the temporary library fixture. The script verifies installation,
service, managed tools, readiness, firewall, uninstall, and ProgramData retention;
the operator records the authenticated scan/playback/Server Manager observations.

Verbose MSI logs and structured `client-result.json` / `server-result.json` remain
beside the scripts. A failed result must not be reported as PASS.
