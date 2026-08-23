# Device sync

Sync uses the existing paired-device access and rotating refresh credentials. There is no API-key architecture. Each device has a monotonically ordered server change feed and submits a stable installation epoch, increasing device sequence, and idempotency event ID. Pulls are bounded to 500 changes. If retained history no longer covers a cursor, `fullResyncRequired` returns the bounded state for that device only.

Progress reconciliation does not trust arrival time or the client clock alone. Explicit `WATCHED` and inferred completion beat incomplete progress. Explicit `UNWATCHED` is a deliberate later device action. For incomplete progress, a parseable client time within 24 hours of server time applies only when newer than the stored playback time; otherwise only forward position is accepted. Thus delayed 40% cannot replace 70%, while a skewed client may still advance progress. Server receipt time is persisted for audit/order.

The same event ID or installation-epoch/sequence pair has one logical effect. Successful progress updates create one sync change and update Continue Watching. Inventory reports contain IDs, sizes, hashes, state, and verification time—not local paths. Storage values are operational client reports, never security evidence. Removal remains `REMOVAL_REQUESTED` until the device acknowledges `REMOVED`.

Revoked accounts, sessions, devices, or DOWNLOAD grants cannot start or resume transfers. A sync removal instruction is provided after grant reconciliation. Bytes already received while a device was disconnected cannot be remotely recalled.
