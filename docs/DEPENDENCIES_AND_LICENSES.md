# Dependencies and licenses

| Dependency | Use | License |
| --- | --- | --- |
| Go standard library | Server/runtime/HTTP/database abstraction | BSD-3-Clause |
| modernc.org/sqlite | CGO-free embedded SQLite driver | BSD-3-Clause |
| golang.org/x/crypto | Argon2id password hashing | BSD-3-Clause |
| modernc SQLite transitive Go modules | SQLite runtime portability (`libc`, `memory`, `mathutil`, Go `x/sys`, `x/exp`, UUID, formatting helpers) | BSD-3-Clause or MIT; exact modules/versions in `go.mod` |
| React / React DOM | Web UI | MIT |
| Vite and @vitejs/plugin-react | Web tooling | MIT |
| TypeScript | Type checking | Apache-2.0 |
| Vitest | Web unit tests | MIT |
| Redocly CLI | OpenAPI validation | MIT |
| Debian Bookworm slim | Minimal container runtime and trusted package base | Debian component licenses |
| FFmpeg / FFprobe Debian package | Read-only technical media inspection | GPL/LGPL depending on Debian build configuration; release notices and source-offer obligations apply |
| Go Alpine image | Container build toolchain | Go BSD-3-Clause plus Alpine licenses |
| GitHub checkout/setup actions | CI checkout and Go/Node setup | MIT |
| Docker Buildx/build-push actions | CI image build validation | Apache-2.0 |

Transitive versions and declared licenses are locked in `go.sum` and
`package-lock.json`; releases must generate a full notices/SBOM report. No Plex,
Jellyfin, or Emby source, assets, protocols, text, or branding are included.
