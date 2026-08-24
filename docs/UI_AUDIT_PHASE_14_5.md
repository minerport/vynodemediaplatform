# Phase 14.5 baseline UI audit

Audit date: 2026-08-23. Baseline: Phase 14 commit `cbfbbb4`, the preserved
Phase 14 runtime, current Android source, and the supplied hierarchy reference.

## Reference qualities to carry forward

The supplied reference is used only for principles: a quiet narrow rail, prominent
search, one controlled cinematic feature region, artwork-forward rows, compact
metadata, and unmistakable Play priority. VyNode will not copy its exact yellow
palette, logo, layout, imagery, icons, or card treatment. VyNode retains its own
warm-orange values, neutral charcoal chrome, account model, and original motion.

## Cross-platform findings

- The current orange-on-neutral identity is promising and distinct, but semantic
  tokens are incomplete and several components still use raw colors.
- Shared primitives remain CSS conventions and two Compose buttons; screens can
  still invent local surfaces and action treatments.
- Consumer and admin density differ in intent, yet the Web rail exposes every
  operational destination alongside everyday media navigation.
- Missing-artwork fallbacks are stable but too typographic and visually dominant.
- Empty, offline, and failure language varies instead of using a shared composition.

## Web baseline

- At 1440×900, the server name is the first and largest visual. “Server A” carries
  more weight than the available movie and provides no everyday value.
- The rail has more than twenty actions and requires its own scrollbar at 900 px.
  Consumer, administration, settings, security, and sign-out compete visually.
- The header repeats page context, account name, a vague Browse action, and server
  status rather than providing first-class search.
- A lone poster stretches into an arbitrary grid fraction. `See all` falls under
  card metadata rather than reading as a row-level action.
- Panels are the default composition for forms and details, creating excess boxes.
- Settings are long independent forms without category-level spatial structure.
- Details have a good backdrop foundation, but secondary actions often inherit the
  filled primary treatment.
- Mobile bottom navigation is intentional, but lacks a controlled icon system and
  exposes Account where the product brief calls for primary Search access.

## Android phone baseline

- Onboarding is coherent but resembles a generic centered form card.
- Home actions are text buttons in a top row rather than a thumb-oriented model.
- Cards need consistent metadata, progress, loading, and fallback treatments.
- Details are functional but do not yet form an intentional narrow-screen cinematic
  composition of artwork, title, actions, metadata, and overview.

## Android TV baseline

- D-pad behavior is proven, but navigation is a Home action row rather than a
  predictable TV-level focus zone.
- Focus is visible on some controls but is not one reusable FocusSurface contract.
- Device code and server selection resemble desktop cards on a large canvas.
- Player behavior is extensive; overlays need hierarchy and motion refinement while
  preserving Media3 `playWhenReady` and all Phase 13 playback semantics.

## Correction order

1. Complete semantic tokens and shared primitives.
2. Simplify navigation and Home hierarchy.
3. Standardize cards, actions, states, and image fallback.
4. Recompose account/settings/admin density.
5. Apply equivalent Compose identity with phone- and TV-native ergonomics.
6. Validate every screenshot and functional preservation gate in running apps.
