# ADR 0001: Phase 0 foundation

Status: accepted

## Decision

Start with one dependency-light Go server process, explicit `/api/v1` routing, a
contract-first OpenAPI document, and a separate React client. Keep handlers thin;
future business capabilities belong in domain/application packages and persistence
behind interfaces. The server is headless and binds to loopback by default.

Phase 0 introduces SQLite-backed migration and identity foundations. Authentication,
WebSocket transport, media discovery, FFmpeg, and playback remain deferred until
their vertical slices can be implemented and tested.

The Docker image builds the same server entry point intended for native Linux and
Windows packaging. Unraid will consume that image rather than a fork.

## Consequences

- The executable proves lifecycle, configuration, routing, errors, readiness, migrations, and identity.
- The web shell exposes only real system information.
- PostgreSQL remains the target production store; Phase 0 implements CGO-free SQLite.
- Local operation has no Internet dependency. The optional web font gracefully
  falls back to system fonts; it should be self-hosted before production release.
