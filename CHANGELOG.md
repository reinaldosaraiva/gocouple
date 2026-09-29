# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
