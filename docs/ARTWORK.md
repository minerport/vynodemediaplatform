# Artwork

Artwork belongs to logical movies, shows, seasons, or episodes. Candidate rows preserve provider path, language, dimensions, ranking, type, selection, and manual-selection state. Supported types are poster, backdrop, logo, season poster, and episode still.

Selected originals are cached below `/config/cache/artwork`; nothing is written beside media. Downloads accept only the controlled HTTPS TMDb image host, reject redirects to other hosts, enforce a 15 MiB limit, validate MIME and decoded image format, generate names from VyNode hashes, and use atomic temporary-file replacement. Failed partial files are removed.

Clients receive images through an authenticated endpoint with MIME, cache control, and ETag. Cache paths are never exposed. Manual selection is represented independently so later metadata refresh does not replace it. Originals are retained for future non-destructive overlay variants; cleanup is intentionally deferred until it can be reference-aware.
