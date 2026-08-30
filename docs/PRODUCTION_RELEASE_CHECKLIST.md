# Production release checklist

## Phase 16 external gate status

- [x] Previously accepted Phase 16 implementation/runtime matrix — **FROZEN / PASS**
- [ ] Production Authenticode — **PENDING**
- [ ] Trusted RFC 3161 timestamp — **PENDING**
- [ ] Signed-artifact native Windows verification — **PENDING**
- [ ] Literal clean Windows client — **PENDING**
- [ ] Literal clean Windows server — **PENDING**
- [ ] Offline literal clean server — **PENDING**

The pending items require external credentials or a genuinely clean Windows
environment. Development evidence must not be substituted for them.

## Build and verification

- [ ] Version and Stable/Beta channel are intentional; no silent downgrade.
- [ ] Go format, vet, tests, Linux/Windows builds, Web, Android, Docker, Windows tests, and both OpenAPI descriptions pass.
- [ ] Desktop and Server MSIs build on a clean Windows runner.
- [ ] Approved FFmpeg archive, FFmpeg, and FFprobe hashes match the pinned manifest.
- [ ] Installed FFprobe scan, Direct Stream, audio transcode, video/HLS, and representative subtitle behavior pass.
- [ ] SQLite integrity and foreign-key checks pass.
- [ ] Source, retained logs, and artifacts pass the secret scan.

## Trust and metadata

- [ ] Protected Authenticode certificate and password are available only to the production release environment.
- [ ] Update public key ID and SPKI are present; private update/signing keys are absent from source and artifacts.
- [ ] First-party executables are signed before MSI construction; both MSIs are signed and timestamped afterward.
- [ ] Authenticode verification passes for every first-party EXE and MSI.
- [ ] Verification report records signer subject/thumbprint, SHA-256 digest,
      trusted timestamp certificate/validity, Windows trust result, and final hash.
- [ ] Final signed Desktop MSI hash is the hash inside signed update metadata.
- [ ] Signed update metadata contains the exact channel, version, HTTPS package URL, package SHA-256, and key ID.
- [ ] Release manifest identifies signed/unsigned state, certificate thumbprint, media tools, SBOM, and every primary artifact.
- [ ] `SHA256SUMS.txt`, CycloneDX SBOM, and third-party notices cover every primary artifact and managed media tools.
- [ ] Unsigned output is labeled `UNSIGNED DEVELOPMENT BUILD`, never production.

## Windows runtime acceptance

- [ ] Clean server machine has no FFmpeg/FFprobe in `PATH`; offline MSI install provides verified managed tools.
- [ ] Service runs as LocalService with delayed automatic startup, scoped recovery, protected Program Files, ProgramData ACL, and scoped TCP 8096 LocalSubnet firewall rule.
- [ ] Actual reboot preserves readiness, database, installation/Ed25519 identity, Connect link, users, grants, libraries, and media operation.
- [ ] Forced service failure triggers bounded SCM recovery without a crash loop.
- [ ] Client clean install reaches global login, Home, and representative playback without an SDK.
- [ ] N→N+1 client update verifies metadata/signature/hash, requests UAC, preserves state, and returns the user to VyNode without elevation.
- [ ] N→N+1 server upgrade replaces managed tools while preserving ProgramData and any explicit custom tool override.
- [ ] Server uninstall removes binaries/service/firewall rule, retains ProgramData/media, and reinstall recognizes preserved identity/configuration.

## Publication

- [ ] Stable client cannot consume Beta metadata unless explicitly rebuilt/switched for Beta.
- [ ] Rollback is a documented manual administrator operation; updater never silently downgrades.
- [ ] Artifact inventory and release documentation match the files being published.
- [ ] Acceptance-only media, databases, certificates, feeds, and credentials remain ignored and are removed after validation.
- [ ] `git diff --check` and final Git classification pass; `.codex/` remains untouched/untracked.
