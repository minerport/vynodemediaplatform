# Implementation roadmap

Phase 7 adds playback preferences, autoplay contexts, Continue Watching controls, manual markers, skip controls, and playback diagnostics. Phase 8 adds conservative local automatic markers, durable manual precedence/review, controlled optimized derivatives, structured automation, scheduling foundations, and unified background work visibility. Phase 9 adds distinct manual/smart collections, private playlists, watchlists, favorites, and per-user typed Home layouts. Phase 10 is server dashboard, playback analytics, library health, notifications, webhooks, and administrative observability.

Phase 5 provides Direct Stream, audio-only transcoding, text subtitles, supervised pipelines, and Continue Watching. Phase 6 is reserved for full video transcoding, adaptive HLS, quality selection, hardware acceleration, HDR/tone-mapping foundations, and resource management.

| Phase | Scope | Exit condition |
| --- | --- | --- |
| 0 — Foundation | Architecture, lifecycle, identity, migrations, API contract, web shell, deployment/CI scaffolds | Tested portable server foundation; no fake media features |
| 1 — Identity and access | Owner bootstrap, login, rotating device sessions, RBAC, audit | Secure offline-capable authentication; recovery remains future work |
| 2 — Libraries and scanning | Roots, guarded scanner, FFprobe adapter, persisted jobs, normalized physical inventory | Complete: real files discovered with progress |
| 3 — Metadata and artwork | Logical movies/TV, deterministic TMDb identification, credits/companies, provenance, persistent artwork cache | Complete: authenticated local-first browsing and offline artwork without playback |
| 4 — Direct Play | Capability profiles, explainable selection, sessions, ranges, progress, browser player | Complete: compatible originals play without conversion |
| 5 — Playback pipeline | Direct Stream/remux, FFmpeg pipeline, audio conversion, subtitle delivery | Expanded browser compatibility with tested reasons |
| 6 — Video transcoding | Software H.264, authenticated fMP4 HLS, quality policy, resource limits, hardware discovery | Validated Chromium HEVC-to-H.264 playback without regressing cheaper modes |
| 7 — Operations and remote | Windows Service/installer, Unraid publication, remote modes, observability | Maintainable deployments |
| 8 — Expanded media | Music, photos, audiobooks, books/comics, downloads, casting | Broader media coverage |
| 9 — Live/collaboration | Live TV/DVR, watch-together, plugins, notifications/webhooks | Advanced ecosystem capabilities |
| 10 — Apple and TV | iOS/iPadOS/tvOS/macOS, then evaluated Roku/Tizen/webOS | Demand-driven native clients |

Every gate requires tests, builds, ADR updates, stated limitations, and no unrelated
refactors. Only the immediately authorized phase begins.
