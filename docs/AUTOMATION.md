# Automation

Automation rules are structured `trigger + conditions + actions`; SQL, scripts, and shell commands are never accepted. Supported triggers are real server events: `MEDIA_ADDED`, `MEDIA_IDENTIFIED`, `METADATA_REFRESHED`, `SCAN_COMPLETED`, and `SCHEDULE`. Conditions are allowlisted media fields. Actions are marker analysis, optimized-version creation, and idempotent `ADD_TO_COLLECTION` / `REMOVE_FROM_COLLECTION` against server-shared manual collections only. Prefer smart collections for live query membership; automation captures static membership.

Dry run evaluates targets and always executes zero actions. Executions are idempotent by rule and event ID, active background jobs are coalesced, and recursion depth is capped at three. Administrative rule changes are security-audited; routine results are stored as automation execution history.

Schedules store an IANA timezone and use persisted minute execution keys, making injected-clock tests deterministic and repeated scheduler ticks safe. No host cron is required.

Resource priority is: active playback, interactive administrative requests, then background analysis/optimization. One background worker waits while video-transcode playback is active. This reserves playback transcode slots; dynamic process pausing after a background encode starts is not claimed.
