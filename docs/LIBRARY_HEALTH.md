# Library health

Health checks are local reevaluations of known inventory state. Implemented categories are `SOURCE_UNAVAILABLE` (ERROR), `MISSING_MEDIA` (WARNING), `PROBE_FAILURE` (WARNING), `UNMATCHED_MEDIA` (INFO), and `STORAGE_LOW` (WARNING below 5 GiB or 5%; ERROR below 1 GiB) for config, transcode, and optimized paths. Codec, resolution, HDR, and subtitle characteristics are inventory filters, never errors.

Issues have stable category/reference identity and move through `OPEN`, `IGNORED`, and `RESOLVED`. Reappearing resolved conditions reopen; ignored issues retain evidence. Ignore/unignore actions are security-audited. When a source is unavailable, one source issue suppresses all per-file missing issues for that source. Recovery resolves the source issue and emits one transition event.

Health evaluation does not contact metadata providers or reprobe files. It can be triggered manually and is designed for bounded scheduling after relevant jobs in future phases.
