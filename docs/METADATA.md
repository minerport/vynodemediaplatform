# Metadata and logical media

Phase 3 keeps filesystem observations in the physical tables and provider claims in logical tables. `media_associations` is the only bridge: a movie can have several file versions, and one TV file can associate with several episodes. Season zero is valid.

TMDb is accessed through the provider interface. Imported records remain authoritative local VyNode records and continue to browse and search when TMDb is unavailable. Provider IDs are alternate identities, never primary keys. Cache entries expire after 24 hours and are disposable.

Identification is an explicit background job after scanning. Existing associations are skipped, so rescans do not churn identity. Manual matching deliberately replaces only associations; unmatching retains physical inventory and marks logical records orphaned only after their last association disappears. Library deletion cascades physical inventory and memberships but does not silently destroy logical metadata history.

Configuration uses `VYNODE_TMDB_TOKEN` or a mode-0600 secret at `/config/secrets/tmdb.token` (relative to the configured application directory). APIs report only configured/not configured. Language defaults to `en-US` and region to `US`; both are configurable.

Standard aired ordering is the only TV ordering in Phase 3. Playback, NFO writing, and media-folder writes are absent.
