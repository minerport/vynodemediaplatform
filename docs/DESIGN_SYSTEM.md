# VyNode Media design system

VyNode is a dark-first media platform with two deliberately different densities: an immersive, media-first consumer experience and a compact, information-rich administration experience. Both use the same semantic colors, spacing, typography, controls, focus treatment, and motion rules.

## Principles

- Media and the next useful action lead consumer screens; server mechanics do not.
- Administration favors scanability, compact tables, aligned metrics, and predictable actions.
- Phone, web, tablet, and TV share information architecture without copying layouts between platforms.
- Every screen has one clear primary action. Secondary and destructive actions never compete with it.
- UI exposes only implemented capabilities. Diagnostics use progressive disclosure.

## Tokens

The canonical conceptual tokens live in `packages/design-tokens/tokens.css`; Compose equivalents live in `apps/android/app/src/main/java/com/vynode/media/DesignSystem.kt`.

### Color

- Background: `#080D0F`; subtle background: `#0D1416`.
- Surface: `#121B1E`; raised: `#192428`; overlay: `#202D31`.
- Text: `#F4F7F7`; muted: `#A8B5B9`; subtle: `#74858B`.
- VyNode accent: `#63D7BD`; high emphasis: `#8CE8D3`; accent ink: `#05251F`.
- Danger: `#FF786C`; warning: `#E9BD68`; focus: `#A8F4E3`.

The accent is reserved for primary actions, selected navigation, progress, and short status cues. It is not a decorative gradient applied indiscriminately.

### Spacing and shape

Spacing follows 4, 8, 12, 16, 20, 24, 32, 40, 48, and 64 units. Radius names are small (8), medium (12), large (18), and pill. Screen-specific one-off spacing should be replaced with the nearest token.

### Typography

Display is reserved for media titles and pairing codes. Page titles identify destinations. Section titles lead rows or admin groups. Media title, body, metadata, caption, and button styles descend in that order. Metadata is muted, never smaller than practical reading size, and never used as the only carrier of important state.

## Components

### Buttons

- Primary: filled accent, dark ink, pill shape; Play, Resume, Continue, Save.
- Secondary: outlined neutral surface; Watchlist, Favorite, Download, Back.
- Ghost: navigation and low-priority context actions.
- Destructive: transparent/dark surface with danger border and text.
- Icon action: only for universally understood actions and always has an accessible label.
- Player action: remote/touch target at least 48dp; uses the same focus ring as browsing.

All variants define disabled opacity, pressed movement, keyboard/remote focus, and consistent horizontal padding. Loading replaces or supplements the label without changing button width unexpectedly.

### Inputs and forms

Inputs use a consistent 42–48 unit height, visible label, concise help, inline validation, and a practical maximum form width. Save placement remains at the end of the logical form. Checkbox labels include the full clickable explanation.

### Media cards

Shared variants are Poster, Landscape/Episode, Compact, Continue Watching, and Collection. Cards use fixed artwork ratios, two-line title limits, muted secondary metadata, subtle progress, and a high-contrast focus ring. TV focus may scale to 1.035 but must not reflow the row.

### Navigation

- Web desktop: persistent compact rail grouped into Browse, Administration, and Settings.
- Web mobile: five-item bottom navigation for Home, Movies, Shows, Downloads, and Account.
- Android phone: native compact top-level destinations; secondary destinations remain contextual.
- Android TV: artwork rows, a small unobtrusive header, and explicit D-pad focus.

Administration never appears as an undifferentiated extension of the consumer Browse list.

### Dialogs, states, and tables

Dialogs use small/medium/large sizes and one footer action hierarchy; dangerous operations require explicit confirmation. Empty states use one short explanation and at most one relevant action. Loading uses layout-shaped skeletons. Admin tables use compact rows, uppercase muted headers, right/contextual actions, and horizontal overflow only as a last-resort mobile fallback.

## Media details and player

Details use a backdrop readability fade, a constrained metadata column, a prominent title, and actions immediately beneath the primary metadata. Technical versions and marker tooling live lower on the page.

The player prioritizes timeline, time, play/pause, seek, Quality, Audio, and Subtitles. Skip Intro/Credits sits above the control surface at the lower edge. Up Next shows episode context, countdown, Play Now, and Cancel without covering the central image. Diagnostics remain secondary.

## Responsive behavior

- 1440/1920 desktop: compact rail, dense media grid, wide detail treatment.
- 1024 tablet: reduced rail and two-column detail only where useful.
- 390/360 mobile: bottom navigation, two-column posters, single-column forms and admin rows.
- 1920 TV: 64dp gutters, 18dp row gaps, large readable section titles, 48dp+ controls, strong focus.

Breakpoints must be inspected for alignment, wrapping, clipped artwork, focus visibility, and wasted space—not only for crashes.

## Accessibility and motion

Keyboard and D-pad focus uses the dedicated focus color and a 3–4px ring. Buttons remain semantic buttons; labels and content descriptions are required when meaning is not visible text. Motion uses 120ms fast, 220ms standard, and 360ms slow durations, communicates state, respects reduced-motion settings, and avoids layout-shifting focus animation.

## Inspiration boundary

VyNode adopts proven media-product principles—quick source access, consistent cross-device hierarchy, artwork-rich details, horizontal TV rows, and contextual actions—without copying Plex branding, assets, dimensions, text, CSS, or screen composition.
