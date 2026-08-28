# Phase 15 release readiness

Updated: 2026-08-26

This document records non-elevated release-readiness findings. It does not replace
the mandatory interactive Windows installation and service acceptance gate.

## OpenAPI advisory triage

The inherited baseline contained 77 Spectral advisories:

| Rule | Count | Assessment | Disposition |
|---|---:|---|---|
| `operation-4xx-response` in Media Server API | 55 | Mixed: useful documentation gaps, but several read/health operations have no meaningful documented client-error contract | Retained for endpoint-by-endpoint API documentation work; no fabricated responses |
| `operation-4xx-response` in Connect API | 5 | Four authenticated routes omitted their known 401 response; device-code creation omitted its known 429 response | Corrected descriptions only |
| `operation-operationId` in Connect API | 17 | Genuine documentation/client-generation gap | Added stable operation IDs only |

The safe Connect corrections change documentation only. They do not alter routing,
handlers, status codes, or runtime schemas. The remaining 55 advisories are not a
Phase 15 runtime defect and should be resolved only alongside verified endpoint
contracts rather than by adding fictional generic errors.

## Web bundle composition

The production Web build produced one 881.83 KB minified JavaScript entry chunk
(265.51 KB gzip), 33.01 KB CSS (7.51 KB gzip), and a 2,357.93 KB source map.
Source-map source content attributes the principal code to:

| Source | Source bytes | Approximate share |
|---|---:|---:|
| HLS.js bundled source | 592,195 | 44.7% |
| React DOM client | 536,016 | 40.5% |
| `App.tsx` | 123,031 | 9.3% |
| `api.ts` | 34,285 | 2.6% |
| Other React/application sources | remainder | 2.9% |

HLS.js is the clearest future split boundary because it is required only when the
player chooses HLS. A dynamic import at that playback boundary would defer it;
merely assigning the static import to a manual chunk would not defer the request.
No optimization was made in this pass because Playback is frozen and the measured
acceptance path had no observed long tasks. A broader `App.tsx` route split can be
considered later with dedicated Web regression coverage.

## Source hygiene inventory

- No TODO, FIXME, HACK, or XXX markers were found in active source.
- `connect.example.invalid` is an intentional non-routable example; localhost
  defaults are development configuration; `127.0.0.1:18443` and `updates.test`
  occur only in acceptance/tests.
- No active mint or green branding was found. Remaining green references describe
  semantic success color or explicitly prohibit the obsolete mint identity.
- Windows update test switches are explicitly named `VYNODE_UPDATE_TEST_*` and
  production behavior remains fail-closed. `DEBUG` conditionals are build-scoped.
- Generated `bin`, `obj`, `dist`, installer output, runtime artifacts, and local
  acceptance state remain ignored rather than tracked.
- Windows documentation has distinct architecture, client, server, installation,
  update-security, troubleshooting, release-note, and checklist scopes. The
  deploy README is a build entry point, not a competing product specification.
- No uncertain source or dependency was deleted.

## Dependency review

- Web production dependencies report zero npm audit vulnerabilities.
- Windows client direct packages are Windows App SDK 2.4.0 and Windows SDK Build
  Tools 10.0.28000.2526; Server Manager uses ServiceController 10.0.0; tests use
  Microsoft.NET.Test.Sdk 17.14.1 and MSTest 4.0.1.
- WiX Toolset 7.0.0 extensions for Firewall and Util are both used by authored
  installer behavior. No unused WiX extension was identified.
- No speculative package upgrade is justified by this audit.
- Web package configuration uses `latest` ranges for React, React DOM, Vite,
  TypeScript, and the React plugin. A future controlled dependency-maintenance
  change should pin reviewed ranges and move build-only tools to devDependencies;
  changing lock resolution during release closeout is not warranted.

## Artifact and acceptance reporting

`deploy/windows/acceptance/New-ArtifactInventory.ps1` emits the machine-readable
release manifest. `New-FinalAcceptanceReport.ps1` combines package, SCM, ACL,
firewall, identity, upgrade, uninstall/reinstall, update-handoff, hash, and cleanup
results into one final structured report. The elevated orchestrator refreshes
that report after every stage so a partial local run remains diagnosable.

## Release decision

Consumer and Playback packets are frozen and passing. Platform implementation and
the update runtime gap are complete. Release artifacts remain unsigned development
builds. Phase 15 cannot be marked complete until interactive elevated runtime
acceptance passes.
