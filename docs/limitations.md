# Current limitations

Phase 0 is foundational and not a usable media server.

- SQLite migrations provide only foundation tables; PostgreSQL adapter support is not implemented yet.
- No authentication behavior, libraries, scanning, metadata, or playback.
- WebSocket and job interfaces exist, but no transport or executor is active.
- No FFmpeg integration, transcoding, or hardware discovery.
- No Windows Service installer; the Unraid template is not yet published or platform-tested.
- The OpenAPI contract covers only implemented version and system-info operations.
- The web client is an honest server status/identity shell, not a library interface.
- Go tests/builds require Go 1.24+, which must be installed in the development environment.

The recommended Phase 1 is local owner bootstrap and device-session authentication.
