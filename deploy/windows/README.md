# Native Windows foundation

VyNode Server is a normal Go executable and does not require Docker. With Go 1.24+:

```powershell
./deploy/windows/build.ps1
$env:VYNODE_CONFIG_DIR = "$env:ProgramData\VyNode Media"
./artifacts/windows/vynode-server.exe
```

The process handles Ctrl+C and service stop signals through graceful shutdown.
A Windows Service wrapper, signed installer, upgrades, and uninstall behavior are
planned; this folder does not claim those packages exist.

