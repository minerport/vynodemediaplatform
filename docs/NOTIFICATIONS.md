# Notifications

Notifications are provider-neutral: typed operational events fan out through subscriptions to destinations and create durable delivery rows. `WEBHOOK` is the initial destination transport. The catalog exposes events wired to real transitions, including server/source/health events plus `INVITATION_ACCEPTED`, `NEW_DEVICE_PAIRED`, `REMOTE_ENDPOINT_CHANGED`, LAN discovery lifecycle events, `PORT_MAPPING_ESTABLISHED`, `PORT_MAPPING_FAILED`, `DOWNLOAD_READY`, `DOWNLOAD_FAILED`, and `DOWNLOAD_CACHE_LOW_SPACE`. Download notifications are lifecycle-level only; transfer-byte progress is deliberately excluded. Payloads never include invitation tokens, pairing challenges, passwords, refresh credentials, or filesystem paths.

Delivery is asynchronous with bounded concurrency (one worker initially), a five-second request timeout, no redirects, and at most five attempts. Network failures, 408, 429, and 5xx retry with short exponential backoff; other 4xx fail immediately. Pending rows survive restart. Disabling or deleting a destination cancels/removes pending work. Response bodies are discarded after a 4 KiB bound and never persisted.

Transition dedupe keys prevent unchanged health conditions from notifying repeatedly. Delivery rows are unique per destination/event. Secrets are encrypted with a local 256-bit installation key and never returned.
