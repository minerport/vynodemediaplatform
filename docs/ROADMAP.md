# Implementation roadmap

| Phase | Scope | Exit condition |
| --- | --- | --- |
| 0 — Foundation | Architecture, lifecycle, identity, migrations, API contract, web shell, deployment/CI scaffolds | Tested portable server foundation; no fake media features |
| 1 — Identity and access | Owner bootstrap, login, rotating device sessions, RBAC, audit | Secure offline-capable authentication; recovery remains future work |
| 2 — Libraries and scanning | Roots, guarded scanner, FFprobe adapter, persisted jobs, normalized physical inventory | Complete: real files discovered with progress |
| 3 — Metadata and artwork | Logical movies/TV, deterministic TMDb identification, credits/companies, provenance, persistent artwork cache | Complete: authenticated local-first browsing and offline artwork without playback |
| 4 — Playback | Capabilities, explainable decision engine, direct play/remux/HLS lifecycle | Browser playback with tested reasons |
| 5 — Media experience | Movies/TV, progress, search, collections, home rows | Credible local video experience |
| 6 — Android ecosystem | Compose phone/tablet and TV clients with Media3 | Native Android and couch playback |
| 7 — Operations and remote | Windows Service/installer, Unraid publication, remote modes, observability | Maintainable deployments |
| 8 — Expanded media | Music, photos, audiobooks, books/comics, downloads, casting | Broader media coverage |
| 9 — Live/collaboration | Live TV/DVR, watch-together, plugins, notifications/webhooks | Advanced ecosystem capabilities |
| 10 — Apple and TV | iOS/iPadOS/tvOS/macOS, then evaluated Roku/Tizen/webOS | Demand-driven native clients |

Every gate requires tests, builds, ADR updates, stated limitations, and no unrelated
refactors. Only the immediately authorized phase begins.
