# VyNode Media architecture

Phase 6 distinguishes playback sessions, persisted transcode sessions, FFmpeg processes, and server-owned HLS artifacts. A central manager admits a conservative number of software video transcodes while Direct Play remains slot-free.

Playback planning is separate from HTTP delivery. `FFmpegPipeline` owns capability detection, allowlisted arguments, capacity, cancellation, and bounded diagnostics. One logical playback session can own multiple pipeline instances without duplicating progress.

Phase 3 adds a strict three-layer media model: filesystem observations, provider-backed logical media, and replaceable many-to-many associations. See [METADATA.md](METADATA.md), [MATCHING.md](MATCHING.md), and [ARTWORK.md](ARTWORK.md). External connectivity enriches the local database but is never a runtime dependency for browsing.

## System shape and boundaries

One headless Go server owns policy, persistence, scanning, metadata coordination,
playback decisions, and jobs. Browser, Windows, Android, and future Apple/TV clients
use documented versioned HTTP and WebSocket APIs only. Clients never access storage
or duplicate server business rules. Domain/application code remains separate from
HTTP handlers, database adapters, FFmpeg adapters, and platform packaging.

## API and events

REST lives under `/api/v1`; health probes remain unversioned. OpenAPI is canonical,
with additive compatible evolution inside v1 and a new namespace for breaking
changes. Errors contain stable code, safe message, and request ID. Generated clients
will be derived into `packages/generated-client` after contract tooling is selected.

Real-time messages will use authenticated WebSockets with an envelope containing
event type, schema version, sequence/time, and typed payload. The Phase 0 interfaces
decouple publishers/subscribers from the future transport. Resumption, backpressure,
authorization, and watch-together ordering must be designed before exposure.

## Persistence and identity

`database/sql` is the adapter boundary. PostgreSQL is the intended production
database; Phase 0 implements embedded, CGO-free SQLite for simple local startup.
Ordered transactional migrations create only settings, users, devices, sessions,
and audit foundations. Each install receives an RFC 4122 v4 instance ID stored in
`server_settings`, surviving restart and future discovery/Connect enrollment.

## Authentication and security

Phase 1 implements owner bootstrap, Argon2id password hashes, short access tokens,
rotating hashed refresh tokens with replay revocation, device sessions, RBAC, rate limiting,
CSRF rules, and audit events. Trust boundaries are: untrusted client to API; API to
validated application commands; application to persistence/filesystem; and server
to external providers. Paths, secrets, tokens, raw provider responses, and internal
errors never cross the API. The server binds to loopback by default.

## Future media pipeline

Phase 2 implements the physical half of this pipeline: typed libraries and sources,
guarded read-only walking, incremental path/size/mtime comparison, bounded FFprobe,
normalized streams, persistent cancellable scan jobs, and offline-root preservation.
Physical files deliberately remain separate from future logical media entities.

Scanning will crawl configured roots through a guarded filesystem adapter, reject
traversal, fingerprint files, probe through a dedicated FFprobe abstraction, and
submit isolated/idempotent jobs. Provider-neutral media entities will hold stable
VyNode IDs; external IDs and provenance remain separate. Metadata, artwork, local
NFO, and embedded tags use prioritized provider interfaces.

Phase 4 playback requests enter an explainable decision engine evaluating source,
subtitle, HDR/Dolby Vision, bandwidth, user policy, client/device capabilities, and
discovered server/GPU capability. The implemented output is DIRECT_PLAY or UNSUPPORTED
plus reason codes; Direct Stream and Transcode remain future modes. Original bytes are
served by an authenticated single-range HTTP adapter. All future FFmpeg work stays behind one media-process adapter.
Transcode sessions own HLS output, cancellation, throttling, and cleanup.

## Plugins and remote access

Plugins will be versioned, permissioned out-of-process workers or sandboxed modules;
untrusted code will not run in the primary process. Initial remote access is explicit
manual URL/reverse proxy/VPN configuration. UPnP, NAT-PMP, tunnels, Connect, and
relay require opt-in, secure negotiation, revocation, and SSRF-safe egress. Internet
availability never gates local authentication, browsing, playback, or administration.

## Deployment and clients

The same server core builds natively on Linux/Windows and into amd64/arm64 OCI
images; Unraid consumes that image. The OCI image builds and serves the production
React bundle, including SPA fallback for direct protected-route reloads. Native
deployments can serve the same bundle by setting `VYNODE_WEB_DIR`. Windows will add
a Service/installer without requiring Docker. React/TypeScript is the browser client. Android is native
Kotlin/Compose with Media3, including TV focus/remote behavior—not a WebView. Apple
clients will use SwiftUI/AVFoundation. Every client consumes API models and
capability contracts rather than persistence models.
