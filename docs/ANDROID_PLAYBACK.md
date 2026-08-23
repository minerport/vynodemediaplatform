# Android playback

One application-scoped Media3/ExoPlayer architecture handles progressive authenticated media and HLS. Bearer credentials are request headers and never permanent media URLs. The client will derive capabilities from Android codec information and Media3 renderers; it must not assume Shield passthrough or codec support.

TV controls reserve D-pad center for play/pause, left/right for seeking, and Back for closing the current overlay before leaving playback. Server semantic audio/subtitle selections, logical marker seconds, progress reconciliation, and next-item context remain authoritative.

Runtime playback format claims are deferred until an emulator or device has actually advanced playback.
