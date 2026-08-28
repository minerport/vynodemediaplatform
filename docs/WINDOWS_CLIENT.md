# VyNode Desktop

VyNode Desktop is the native WinUI 3 consumer client in
`apps/windows/VyNode.Windows`. It uses the Phase 14 global account protocol and
the Phase 14.5 black/charcoal and orange experience system. The normal first run
is global Sign In; linked servers are fetched automatically. Endpoint entry is
restricted to Advanced recovery and local-only workflows.

The client generates a random application device identifier. It never reads a
hardware serial or machine fingerprint. Global and per-server refresh credentials
are separate Windows Credential Locker records. Access tokens are memory-only.

Build and run on Windows:

```powershell
dotnet build apps/windows/VyNode.Windows/VyNode.Windows.csproj -p:Platform=x64
dotnet run --project apps/windows/VyNode.Windows/VyNode.Windows.csproj -p:Platform=x64
```

`VYNODE_CONNECT_URL` may point development builds at an acceptance Connect
deployment. It is not a server URL and is not exposed as normal client UI.

Windows offline downloads are not exposed until the complete server/account
namespacing, checksum, quota, resume, and logout-privacy contract is implemented.
