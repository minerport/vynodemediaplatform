# Dependencies and licenses

Phase 8 introduces no cloud or new runtime dependency. Local fingerprint extraction, credits sampling, and optimization reuse the existing FFmpeg/FFprobe dependency. Go standard-library SHA-256 provides source and derivative identity checks.

The Debian Bookworm image installs Debian's `ffmpeg` package for FFmpeg and FFprobe. Its exact version is reported by the safe admin capability API. Redistributors must review Debian's FFmpeg build configuration and applicable LGPL/GPL codec obligations.

TMDb is an optional external metadata service accessed through its documented API using an operator-supplied credential. VyNode does not bundle TMDb data or credentials. Image validation uses Go standard-library decoders; Phase 3 adds no runtime package dependency.

| Dependency | Use | License |
| --- | --- | --- |
| Go standard library | Server/runtime/HTTP/database abstraction | BSD-3-Clause |
| modernc.org/sqlite | CGO-free embedded SQLite driver | BSD-3-Clause |
| golang.org/x/crypto | Argon2id password hashing | BSD-3-Clause |
| modernc SQLite transitive Go modules | SQLite runtime portability (`libc`, `memory`, `mathutil`, Go `x/sys`, `x/exp`, UUID, formatting helpers) | BSD-3-Clause or MIT; exact modules/versions in `go.mod` |
| React / React DOM | Web UI | MIT |
| hls.js | Authenticated HLS playback in MSE-capable browsers | Apache-2.0 |
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
# Phase 10 dependency note

Phase 10 adds no third-party runtime or chart dependency. Metrics, HMAC signing, URL/DNS policy, asynchronous delivery, and compact dashboard visualizations use the Go and browser standard libraries.
