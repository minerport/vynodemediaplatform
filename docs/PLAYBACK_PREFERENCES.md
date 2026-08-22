# Playback preferences

VyNode stores preferences per user. Audio and subtitle choices are semantic: normalized language, commentary, forced, hearing-impaired, default, and compatibility attributes are evaluated independently for every physical version. Stream indexes and track IDs are only session-local delivery identifiers.

Precedence is explicit session choice, ordered user languages, non-commentary default, non-commentary usable track, then deterministic fallback. Manual player selection does not modify account settings. Version changes match the selection's semantic attributes in the new version.

Subtitle modes are `OFF`, `ALWAYS`, `WHEN_AUDIO_NOT_PREFERRED`, and `FORCED_ONLY`. Forced behavior depends on correctly tagged media. Text subtitles use native/WebVTT delivery; image subtitles remain unsupported because burn-in is not implemented.

Home and remote quality defaults feed the Phase 6 policy. The strictest server, user, client, and network constraint wins.
