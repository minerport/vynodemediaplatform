# Current limitations

Phase 1 provides local identity and remains intentionally not a usable media server.

- SQLite migrations provide only foundation tables; PostgreSQL adapter support is not implemented yet.
- Local authentication is implemented; MFA, recovery, libraries, scanning, metadata, and playback are not.
- WebSocket and job interfaces exist, but no transport or executor is active.
- No FFmpeg integration, transcoding, or hardware discovery.
- No Windows Service installer; the Unraid template is not yet published or platform-tested.
- The OpenAPI contract covers implemented foundation and identity operations.
- The web client provides setup, login, account security, sessions, users, and audit—not media browsing.
- Native binaries require a separately deployed web bundle and `VYNODE_WEB_DIR`; the Docker image includes it.
- Go tests/builds require Go 1.24+, which must be installed in the development environment.

The media library/scanner phase has not begun.
