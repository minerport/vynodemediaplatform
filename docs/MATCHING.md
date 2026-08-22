# Identification and matching

Matching is deterministic. Normalized exact filename title contributes 70 points, an exact year 25, an exact parent-directory title 10, a contained/similar title 40, and a one-year difference 8.

- High confidence: at least 90 points and a lead of at least 15; auto-match.
- Medium confidence: at least 60 points without the high-confidence margin; ambiguous review.
- Low confidence: below 60; unmatched.

Candidate ordering cannot turn uncertainty into certainty. Scores, candidates, and signal explanations are persisted in `metadata_match_attempts`. TV first identifies the show, then resolves the provider's season and numbered episode; it never performs a global episode-title search. Multi-episode filename ranges create one association per episode.

Manual match and Fix Match use the same explicit association operation and are audited. Manual associations are stable across scans. Unmatch deletes only the association; it never changes or deletes the file.
