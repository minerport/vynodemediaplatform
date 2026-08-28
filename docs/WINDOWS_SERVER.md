# VyNode Media Server on Windows

The Go server supports both console and native Windows Service Control Manager
execution. Console mode remains the development and recovery path. The installed
service name is `VyNodeMediaServer`, uses delayed automatic startup, and runs as
`NT AUTHORITY\LocalService` by default.

- Binaries: `%ProgramFiles%\VyNode\Media Server`
- Persistent data: `%ProgramData%\VyNode\Media Server`
- Logs: `%ProgramData%\VyNode\Media Server\logs\server.log`
- Default HTTP port: 8096

VyNode Server Manager is installed with the server package. It is intentionally
limited to service status and control, installed version, Web Admin launch, and
data/log folder access. Privileged SCM operations use Windows UAC; there is no
custom privileged IPC channel.

LocalService is deliberately low privilege. Local NTFS libraries must grant it
read access. It has anonymous network credentials, so UNC libraries generally need
an explicitly configured service identity with both share and NTFS permissions.
VyNode does not store share passwords in configuration.

The server MSI stops/restarts the service during servicing and preserves ProgramData,
including the SQLite database and Ed25519 identity, across upgrade and uninstall.
