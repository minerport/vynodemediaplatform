# Metadata and logical media

Phase 3 keeps filesystem observations in the physical tables and provider claims in logical tables. `media_associations` is the only bridge: a movie can have several file versions, and one TV file can associate with several episodes. Season zero is valid.

TMDb is accessed through the provider interface. Imported records remain authoritative local VyNode records and continue to browse and search when TMDb is unavailable. Provider IDs are alternate identities, never primary keys. Search/detail provider-cache entries expire after 24 hours and are disposable; browsing never depends on them.

Movie enrichment stores directors, straightforward writing credits, and at most 20 primary cast members. Show enrichment stores creators and a similarly bounded primary cast. People and production companies are normalized by provider identity and associated without duplication. A failed refresh records a bounded diagnostic state while retaining the last-known-good title, overview, genres, external IDs, credits, companies, associations, and artwork.

Identification is an explicit background job after scanning. Jobs report `QUEUED`, `RUNNING`, `COMPLETED`, `COMPLETED_WITH_ERRORS`, or `FAILED` plus examined, matched, ambiguous, unmatched, already-matched, and failure counts. Only one active metadata job is allowed per library; another request receives the active job with HTTP 409. Metadata-job cancellation is not implemented in Phase 3. Existing associations are skipped, so rescans do not churn identity. Manual matching deliberately replaces only associations; unmatching retains physical inventory and marks logical records orphaned only after their last association disappears. Library deletion cascades physical inventory and memberships but does not silently destroy logical metadata history.

Configuration uses `VYNODE_TMDB_TOKEN` or a mode-0600 secret at `/config/secrets/tmdb.token` (relative to the configured application directory). APIs report only configured/not configured. Language defaults to `en-US` and region to `US`; both are configurable.

Standard aired ordering is the only TV ordering in Phase 3. Playback, NFO writing, and media-folder writes are absent.
