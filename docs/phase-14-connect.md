# VyNode Connect architecture

VyNode Connect is an optional identity and server-discovery control plane. It is a separate executable, database, signing-key boundary, container, and deployment profile from VyNode Media. The ordinary media-server image neither embeds nor requires Connect. Connect stores accounts, global devices, server descriptors, links, invitations, and audit events; it never stores library metadata and never proxies media.

## Trust and session flow

1. A media server owns a stable Ed25519 identity and registers a public key and endpoint descriptors.
2. A signed claim intent lets a signed-in global account claim the server. An existing local user must explicitly link that global identity while authenticated locally.
3. Connect issues an Ed25519 assertion lasting two minutes with issuer, server audience, global account, global device, and random nonce claims.
4. The media server validates the assertion against locally pinned Connect keys, checks issuer/audience/time, requires an active local link, atomically consumes the nonce, checks the local user is active, and issues a normal server-local session.

Global invitations are also server-authoritative. The server first persists the intended role and library grants, registers only a signed opaque digest with Connect, and gives the recipient a bearer invitation. Acceptance binds that invitation to one global account and produces a short-lived, audience-bound, single-use redemption assertion. In one local transaction the target server rechecks its current intent digest, creates a `GLOBAL_LINKED` principal without a reusable password, applies its own role/grants, records the explicit global link, and consumes the JTI. Connect never supplies authoritative library IDs or permissions.

Every assertion-created local session is mapped to the global device that bootstrapped it. Claimed servers use their Ed25519 identity to pull the current durable revoked-device set after each heartbeat. Matching local sessions are revoked and audited. An offline server therefore applies an earlier global revocation when connectivity returns; Connect does not need to reach into or mutate a server database.

Connect assertions are bootstrap credentials only. They are not accepted by library or playback APIs. Local passwords, sessions, downloads, discovery, and LAN reconnects continue to operate when Connect is unavailable.

On Android, global sign-out clears every local server credential, offline file, download record, queued progress event, and sync cursor belonging to that global account's linked server profiles. This intentionally favors account-switch privacy over retaining downloaded bytes. Phase 13 manual/local profiles remain independent of global sign-out.

## Privacy and availability

Connect receives endpoint health, global device, and relationship information only. It does not query media APIs. Clients cache server descriptors but connect directly to selected media servers and verify the stable server ID. Endpoint URLs must be HTTPS outside explicit development configuration. Removing a global link does not delete the local account or media data.

## Threat model

- Passwords use Argon2id with unique salts. Global refresh tokens rotate and reuse revokes their family/device.
- Registration, claims, endpoint changes, and heartbeats are signed by the server identity.
- Invitation registration/revocation and device-revocation pulls are signed by the same persistent server identity.
- Assertion replay, wrong audience, wrong issuer, expired assertion, unknown key, unlinked account, and disabled local account all fail closed.
- Device codes are high-entropy, short-lived, poll-throttled, single-use records; their user-facing code is stored hashed.
- Login and registration are rate limited and return structured, enumeration-resistant errors.
- Invitation and token values are hashed at rest and must be redacted from request logging.
- A Connect compromise cannot grant library permissions: the local server remains the authorization authority.

## Development

Run `docker compose -f deploy/compose/docker-compose.connect.yml up --build`. Connect listens on port 8090 and persists only in the `connect-data` volume. The existing `deploy/compose/docker-compose.yml` remains unchanged and starts only the local media server.
