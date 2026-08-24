# VyNode user-interface inventory

This inventory is the Phase 14.5 preservation map. **Implemented** means the
surface exists now. **Partial** means the capability is embedded in another
screen. **Future** means no honest product surface or supporting flow exists;
Phase 14.5 must not simulate it.

## Global account and onboarding

| Surface | Web/server | Android phone | Android TV |
| --- | --- | --- | --- |
| Welcome and global sign-in | Connect Web and server handoff | Implemented | Device-code welcome |
| Create account | Connect Web | Implemented | Completed in browser through device code |
| No linked server | Connect Web | Server picker | Server picker |
| Linked servers | Connect Web | Implemented | Implemented |
| Devices | Connect Web | Current device/session | Current device/session |
| Invitation acceptance | Connect Web and server redemption | Browser handoff | Browser handoff |
| Device-code approval | Server Web account page | Not applicable | Implemented |
| Manual/local fallback | Server login | Advanced | Advanced |

## Consumer Web

- Home and configurable Home rows
- Movies, movie detail, technical versions, artwork, markers, and playback actions
- Shows, show detail, seasons, episodes, and playback actions
- Collections, collection detail, playlists, watchlist, and favorites
- Downloads/device assignments
- Account, playback preferences, sessions, device pairing, and Home settings
- Search is currently a browse affordance rather than a dedicated grouped-search route

The Web client has no separate season or episode URL. Those surfaces are embedded
in show detail and the player. Server selection and global account management live
in Phase 14 Connect/onboarding rather than the local media-server shell.

## Administration

- Dashboard, playback analytics, active streams, jobs, library health
- Libraries and sources, metadata/provider settings, unmatched-media review
- Users, roles, library grants, sharing, invitations, and device pairing
- Remote access, discovery, automation, webhooks, and audit
- Account/server settings and playback preferences
- Connect linkage is part of Phase 14 account/linking rather than a second
  cryptographic-management dashboard

## Player

- Direct Play, Direct Stream, audio transcode, video transcode, and HLS
- Play/pause, seek, progress, resume, Start Over, quality, audio, subtitles
- Skip Intro, Skip Credits, Up Next, autoplay, buffering, errors, diagnostics
- Fullscreen and native offline playback

## Android phone

- Global sign-in/registration, zero-server state, server picker, manual fallback
- Home rows, search, movie detail, show/season/episode presentation
- Offline downloads and offline player
- Native player with tracks, quality, markers, and Up Next
- Account and server switching are Home actions. Dedicated Movies, Shows,
  Settings, and Account destinations are future, not hidden completed screens.

## Android TV

- Device-code onboarding, zero-server state, server picker, manual pairing
- Home rows, search, movie detail, show/season/episode presentation
- Native player, tracks, quality, markers, Up Next, and server switching
- D-pad focus and restoration
- Dedicated library and account destinations are future; access currently occurs
  through Home, search, server selection, and sign-out actions.

## States

All platforms distinguish loading, empty, error, server offline, Connect
unavailable, device offline, locally downloaded media, missing artwork, playback
buffering, and playback failure. Empty variants include no media, search results,
downloads, collections, linked servers, streams, jobs, or invitations.

