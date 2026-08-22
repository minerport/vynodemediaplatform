# Collections

VyNode keeps manual and smart collections as separate domains. Manual collections store unique, typed Movie/Show memberships and may use custom, title, year, added-date, release-date, or rating order. Deleting one removes only its curation records; logical metadata, artwork, versions, and physical files are untouched.

Smart collections store a schema-versioned, validated rule tree. `ALL`, `ANY`, and bounded `NOT` nodes compile to parameterized allowlisted SQL. Supported fields include title, year, genre, rating, content rating, release/add dates, availability, resolution, video/audio codec, channels, HDR, optimized state, watched/progress state, and favorite state. Results evaluate on request, so the saved query remains authoritative and user-dependent fields use the requesting user context.

Scopes are `SERVER_SHARED` and `USER_PRIVATE`. Administrators manage shared definitions. Private definitions are owner-only. Collection artwork uses deterministic existing-item fallback in Phase 9; custom upload and mosaic generation remain future enhancements.
