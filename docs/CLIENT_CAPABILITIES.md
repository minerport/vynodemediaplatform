# Client capability profiles

Playback requests may include a quality identifier and bandwidth ceiling. The server combines these with verified codec, resolution, and server-side network context; clients cannot force an unavailable backend or unsafe output.

Schema version 2 adds `fragmentedMp4Support` and `maximumAudioChannels`. The browser profile reports fMP4 only with detected MP4 support and does not generically claim HEVC. Future native clients can truthfully advertise MKV, HEVC, DTS, TrueHD, or PGS and receive Direct Play.

Every playback request carries schema version 1 of a reusable capability profile. It includes client and platform identity, containers, video/audio codecs, resolution limits, HDR modes, subtitle formats, and a Direct Play switch. Profiles affect compatibility only and never grant authorization.

The browser adapter uses `HTMLMediaElement.canPlayType()` for conservative MP4 and WebM declarations. Unknown support is omitted rather than guessed from the User-Agent. HDR is not reported by the initial web adapter. Embedded track selection, external subtitles, Dolby Vision conversion, and tone mapping are not claimed.

Profiles are persisted with the user and login-session identity for playback diagnostics, but each new playback request submits a fresh profile because browsers and native apps change. Future Windows, Android/Android TV, Apple, and television clients use the same versioned contract and decision engine.

