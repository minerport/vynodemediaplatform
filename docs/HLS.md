# Authenticated HLS

Video transcodes produce a single selected-quality fMP4 HLS presentation. FFmpeg writes four-second segments, `init.mp4`, and an event playlist under the configured transcode root at `vynode/<playback-session-id>`. This is an ABR-ready layout, but Phase 6 does not claim a multi-rendition ladder or low-latency HLS.

The Go server authorizes every playlist, initialization fragment, and segment against the user, login session, playback session, short-lived media credential, and enabled-user state. Resources use private `no-store` caching. URLs contain no media path; segment names are server-generated and traversal is rejected.

Only directories containing VyNode's ownership marker are removed. Startup cleanup preserves unrelated transcode-root content. Stop, admin termination, shutdown, and stale cleanup cancel FFmpeg and remove owned artifacts.

The web player keeps the HTML video element and uses hls.js on browsers without native HLS. HLS timeline seeking supplies forward/backward seek without Phase 5 progressive-stream replacement.
