# Observability

Phase 10 adds an OWNER/ADMIN-only operational console backed exclusively by local server data. Dashboard live widgets poll every five seconds; historical analytics load independently and use indexed UTC ranges capped at 366 days. VyNode sends no telemetry and requires no Internet connection.

Current live metrics are uptime, OS/architecture, Go heap and reserved runtime memory, goroutine count, active playback sessions, active FFmpeg pipelines, and total/used/available bytes for configured config, transcode, and optimized filesystems. Linux additionally reports process RSS from `/proc/self/status` and system/container-visible total and available memory from `/proc/meminfo`; unsupported platforms omit those values. Filesystem totals are shown per configured path and identical normalized paths are not repeated.

Playback analytics aggregate real playback sessions by delivery mode, completion/error state, logical type, user, and bounded time range. OWNER/ADMIN receive server-wide data; the account summary endpoint always derives the user from the authenticated principal. Operational events are separate from security audit and retained for 90 days; completed webhook deliveries are retained for 30 days. Cleanup never deletes security audit records.

The Phase 10 live strategy is efficient polling. SSE/WebSocket delivery is deferred; high-frequency samples are not persisted in SQLite.
# Offline downloads

The Downloads disk is measured independently from original media. Download preparation appears in the unified job model, and durable `DOWNLOAD_READY`, `DOWNLOAD_FAILED`, and `DOWNLOAD_CACHE_LOW_SPACE` events avoid per-byte noise. Detectable unwritable, low-space, corrupt, and repeated preparation failures surface as health issues and resolve when the condition clears.
