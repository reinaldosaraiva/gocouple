# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [0.3.0] - 2026-09-30

### Added

- Opt-in volatility from the git history: `--volatility-since <duration|date|RFC3339>` on `analyze` and `check`, and `volatility.since` in `.gocouple.yaml`. Every package gets `churn` and `volatility` (omitted from the JSON when zero), and the table, csv and markdown output show a `CHURN` column; `report` shows it when the snapshot it renders was produced with volatility.
- A `pain-zone` finding on a package with no commit in the window moves to `Suppressed` with the reason `stable in window (no commit since <date>)`; with commits it keeps its severity and the message states how many times the package changed. Dormant findings no longer count toward `max_pain_packages`, the `max_distance` ceiling or a `--baseline` regression.
- Not being in a git work tree, or being in a shallow clone, is a warning and the analysis continues without volatility.

### Changed

- `history` ignores `volatility.since` and prints a note.
- Without the flag the output is byte-identical to 0.2.1 and a 0.2.1 `analysis.json` still loads as a baseline.

## [0.2.1] - 2026-09-30

### Added

- The HTML report lists suppressed findings, with their reason, under the diagnostics of each frame.
- An `ignore` entry that silences no finding is reported as a warning (`warnings` in the JSON, `Warnings` in table, markdown and the report, stderr in `check`); the exit code does not change.

### Changed

- `docs/metrics.md` records that a file carrying the generated header next to hand-written code counts as generated.
- The release workflow fires only on full semver tags, so the moving major tag `v0` (documented in `docs/decisions.md`) never triggers a release.

## [0.2.0] - 2026-09-29

### Added

- Packages whose only declarations live in `// Code generated ... DO NOT EDIT.` files are marked `generated` in the JSON; their diagnostics are suppressed and they are exempt from `check` `max_distance`. Generated interfaces are skipped by `wasted-abstraction`.
- `ignore` entries in `.gocouple.yaml` (`rule`, `package` glob, mandatory `reason`) silence one diagnostic rule while keeping the package measured.
- Silenced findings are reported under `suppressed` in the JSON and in a `Suppressed` section of the table and markdown output.

## [0.1.0] - 2026-09-29

### Added

- `analyze`: Ca, Ce, instability, abstractness, distance and zone per package; table, json, csv, markdown, dot and mermaid output.
- Diagnostics: `pain-zone`, `uselessness-zone`, `god-package`, `dependency-cycle`, `concrete-hotspot`, `sdp-violation` and `wasted-abstraction` (go/types based).
- `history`: analysis of commits in detached git worktrees with a configuration-aware cache, parallel workers and clean-up on interruption; json and csv output.
- `report`: self-contained HTML with the A x I chart, commit slider and package trail, time series, dependency graph, sortable tables, light and dark themes.
- `check`: thresholds, baseline mode, GitHub Actions annotations and exit codes 0, 1 and 2.
- `.gocouple.yaml` configuration with strict validation.
- Composite GitHub Action, CI workflow, and a release pipeline with signed checksums, SBOMs and build provenance.
