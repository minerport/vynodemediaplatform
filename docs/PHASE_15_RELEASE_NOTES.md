# VyNode Phase 15 release notes

Status: release candidate; interactive elevated Windows acceptance remains pending.

Phase 15 introduces the first native Windows VyNode experience. The WinUI desktop
client uses the same black/charcoal and warm-orange identity as Web and Android,
with native Home, Movies, Shows, Search, details, Account, and Settings.

## Highlights

- Global VyNode Account sign-in discovers linked servers without requiring an IP
  address, hostname, port, or URL in the normal flow.
- Multi-server switching keeps independent server sessions, roles, grants, and
  media state. Manual connection remains an Advanced recovery path.
- Native Windows playback supports Direct Play, Direct Stream, audio transcode,
  HLS/video transcode, quality, audio, subtitles, markers, Up Next,
  resume/start-over, fullscreen, progress, and retry.
- VyNode Media Server supports Windows SCM service and console modes, persistent
  ProgramData state, delayed automatic startup, LocalService, and an
  executable-scoped local-subnet firewall rule.
- Server Manager provides service control, version, Web Admin, data, and log paths.
- Independent WiX MSIs provide client-only and server-only packages.
- Update runtime verifies ECDSA P-256 signed metadata using a build-pinned public
  key, rejects downgrades and modified packages, re-hashes the managed MSI before
  explicit Windows Installer handoff, and never bypasses UAC.
- Global and server credentials remain isolated in Windows Credential Locker.
  Server identity is verified before local credentials are transmitted.

## Development-build status

Current artifacts are UNSIGNED DEVELOPMENT BUILD outputs. SmartScreen or publisher
warnings are expected. Production publishing requires externally provided
Authenticode credentials and the official update public key. No private signing
material is stored in the repository.

## Acceptance status

Consumer and native Playback packets are frozen and passing. Platform code,
packages, update trust, documentation, and remote-safe harnesses are prepared.
Phase 15 is not final until clean install, SCM, firewall, ACL, upgrade,
uninstall/reinstall, reboot, and update-handoff checks run interactively with UAC.

