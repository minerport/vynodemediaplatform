# Current limitations

Phase 2 provides physical inventory and remains intentionally not a playback or identified-metadata server.

- SQLite migrations provide authentication and physical inventory; PostgreSQL is not implemented.
- Local authentication, libraries, scanning, and FFprobe inspection are implemented; MFA, recovery, metadata identification, and playback are not.
- Scan jobs and polling are active; WebSocket event transport is not.
- No playback, transcoding, or hardware discovery.
- No Windows Service installer; the Unraid template is not yet published or platform-tested.
- The OpenAPI contract covers foundation, identity, library, scan, and inventory operations.
- The web client provides administrative physical inventory, not consumer movie/show browsing.
- Native binaries require a separately deployed web bundle and `VYNODE_WEB_DIR`; the Docker image includes it.
- Go tests/builds require Go 1.24+, which must be installed in the development environment.

Phase 2 adds physical inventory, but metadata identification, artwork, playback,
transcoding, move detection, and real-time WebSocket scan events remain pending.
Windows FFprobe is PATH/configured rather than bundled.
