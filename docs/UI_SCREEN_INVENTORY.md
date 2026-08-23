# Production UI screen inventory

This checklist is the Phase 13 visual-review surface. A checked implementation item means it uses the shared design system; runtime evidence is recorded separately and must not be inferred from this list.

## Web consumer

- Setup, login, and invitation acceptance
- Home and configured Home rows
- Movies, movie detail, Shows, and show detail
- Collections and collection detail
- Playlists, Watchlist, Favorites, and Downloads
- Browser player and playback preferences
- Account, sessions, and Home-row settings

## Web administration

- Dashboard, analytics, health, jobs, and active streams
- Libraries and physical inventory
- Metadata, matching, artwork, marker, and optimization tools
- Users, sharing, invitations, pairing/devices, and audit
- Remote access, automation, and webhooks/notifications

Admin pages share compact headers, metrics, forms, tables, filters, row actions, and severity states. They intentionally remain denser than consumer pages.

## Android phone/tablet

- Connection, insecure-local confirmation, and pairing
- Home, Search, Movie detail, Show/Episodes
- Player and player menus
- Downloads, offline library/playback, and error/identity states

## Android TV / Shield target

- TV connection/pairing and credential restore
- Authenticated Home rows and Search
- Movie/Show detail and Episodes
- Direct/HLS player, Quality, Audio, Subtitles, Skip, and Up Next
- Revocation, identity mismatch, and reconnect states

## Required review sizes

- Web: 1440×900, 1920×1080, 1024×768, 390×844, 360×800.
- Android: phone runtime, tablet when available, and 1920×1080 TV runtime.

Review each representative screenshot for alignment, density, button hierarchy, corner consistency, wrapping, clipped artwork, focus visibility, and responsive transitions.
