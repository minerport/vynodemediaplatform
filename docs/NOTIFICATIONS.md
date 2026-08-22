# Notifications

Notifications are provider-neutral: typed operational events fan out through subscriptions to destinations and create durable delivery rows. `WEBHOOK` is the initial destination transport. The Phase 10 catalog exposes only events wired to real transitions: `TEST`, `SERVER_STARTED`, `SOURCE_UNAVAILABLE`, `SOURCE_RECOVERED`, `HEALTH_ERROR_OPENED`, and `HEALTH_ERROR_RESOLVED`. Scan, playback-error, optimization, and automation notification producers are deferred rather than advertising events that are not yet emitted.

Delivery is asynchronous with bounded concurrency (one worker initially), a five-second request timeout, no redirects, and at most five attempts. Network failures, 408, 429, and 5xx retry with short exponential backoff; other 4xx fail immediately. Pending rows survive restart. Disabling or deleting a destination cancels/removes pending work. Response bodies are discarded after a 4 KiB bound and never persisted.

Transition dedupe keys prevent unchanged health conditions from notifying repeatedly. Delivery rows are unique per destination/event. Secrets are encrypted with a local 256-bit installation key and never returned.
