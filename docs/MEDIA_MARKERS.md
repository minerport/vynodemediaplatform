# Media markers

Markers live on the logical media timeline and therefore apply equally to Direct Play, Direct Stream, Audio Transcode, and Video Transcode. Types are `INTRO`, `RECAP`, `CREDITS`, `POST_CREDITS`, and `CUSTOM`; the schema permits multiple ranges of each type.

OWNER and ADMIN users can create, edit, and delete validated manual markers. Changes are security-audited. Phase 7 does not perform automatic detection.

Intro, recap, and credits controls appear only inside the corresponding range and seek to its logical end. Reaching credits counts as watched only when no post-credits marker exists. Without credits data, the 90% fallback remains. Post-credit content is never automatically skipped.
