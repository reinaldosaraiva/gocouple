# gocouple

[![CI](https://github.com/reinaldosaraiva/gocouple/actions/workflows/ci.yml/badge.svg)](https://github.com/reinaldosaraiva/gocouple/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/reinaldosaraiva/gocouple.svg)](https://pkg.go.dev/github.com/reinaldosaraiva/gocouple)
[![Go Report Card](https://goreportcard.com/badge/github.com/reinaldosaraiva/gocouple)](https://goreportcard.com/report/github.com/reinaldosaraiva/gocouple)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/reinaldosaraiva/gocouple)](https://github.com/reinaldosaraiva/gocouple/releases)

Coupling metrics and architectural health over time for Go. English | [Português](README.pt-BR.md)

`gocouple` measures how packages of a Go module depend on each other (Ca, Ce, instability, abstractness, distance from the main sequence, cycles), diagnoses the patterns that hurt, tracks them across the git history, renders a self-contained HTML report, and fails your CI when the architecture gets worse.

## Why measure coupling continuously

Coupling metrics are old and useful, but nobody computes them by hand, so they go stale with every feature. `gocouple` makes the architecture observable: a number you can diff in a pull request, a trend you can look at, and a gate that stops regressions. It also gives humans and AI agents a shared, objective definition of "this refactoring is done".

## Install

```bash
go install github.com/reinaldosaraiva/gocouple/cmd/gocouple@latest
```

Or download a binary for Linux, macOS or Windows (amd64, arm64) from the [releases page](https://github.com/reinaldosaraiva/gocouple/releases) and see [Verifying releases](#verifying-releases).

## Quick start

```bash
gocouple analyze ./...                       # table with metrics, zones and diagnostics
gocouple analyze --format json --out analysis.json ./...
gocouple history --max 30 --out history.json # analyze the last 30 commits, cached
gocouple report --history history.json --out report.html
gocouple check ./...                         # exit 0 ok, 1 violation, 2 error
```

Formats for `analyze`: `table`, `json`, `csv`, `markdown` (ready for a pull request comment), `dot` and `mermaid` (dependency graph). Try the [example report](examples/report.html) without installing anything; it shows a small service getting worse over three commits and then being refactored.

## Metrics and zones

| Metric | Meaning |
|--------|---------|
| Ca | packages that depend on this one |
| Ce | packages this one depends on |
| I | instability, `Ce / (Ca + Ce)` |
| A | abstractness, interfaces / named types |
| D | distance from the main sequence, `abs(A + I - 1)` |

On the A x I diagram, the main sequence is the line from (I 0, A 1) to (I 1, A 0). Packages close to it balance stability and abstraction. A stable, concrete package that many others use (lower left) is in the **zone of pain**: changing it hurts everyone. An abstract package nobody depends on (upper right) is in the **zone of uselessness**. Full definitions, thresholds and known limitations are in [docs/metrics.md](docs/metrics.md).

Diagnostics: `pain-zone`, `uselessness-zone`, `god-package`, `dependency-cycle`, `concrete-hotspot`, `sdp-violation` (stable depends on unstable) and `wasted-abstraction` (an interface nobody consumes, or whose consumers still depend on the implementation), the last one based on `go/types`.

## Before and after

`testdata/orders-legacy` is a deliberately problematic order system; `testdata/orders-refactored` is the same domain after putting the logger behind an injected interface, depending on contracts and composing in `cmd/`. Real output:

```text
$ gocouple analyze --dir testdata/orders-legacy ./...
PACKAGE              Ca  Ce  I     Na  Nc  A     D     ZONE
internal/contracts   0   1   1.00  2   2   1.00  1.00  uselessness
internal/logger      10  0   0.00  0   1   0.00  1.00  pain
internal/promo       2   1   0.33  0   0   0.00  0.67  pain
...
Summary: packages=12 avg_distance=0.41 pain=2 uselessness=1 main_sequence=9 isolated=0 cycles=0

Diagnostics (8)
[warning] pain-zone internal/logger: stable and concrete package used by 10 packages; introduce a contract ...
[warning] concrete-hotspot internal/logger: package without abstractions is imported directly by 10 packages
[warning] god-package internal/order: package imports 10 packages (threshold 8) and knows too much
[warning] wasted-abstraction internal/payment: interface Gateway exists, but consumers depend on its implementation ...
[warning] wasted-abstraction internal/notify: interface Notifier has no consumers outside its package
...

$ gocouple analyze --dir testdata/orders-refactored ./...
Summary: packages=7 avg_distance=0.39 pain=0 uselessness=0 main_sequence=7 isolated=0 cycles=0

Diagnostics (0)
```

![Report of the legacy system: A x I chart with two packages in the zone of pain](docs/images/report-legacy.png)

The full HTML of the legacy analysis is in [examples/orders-legacy.html](examples/orders-legacy.html).

## Using with AI agents

- Give an agent the analysis as context before it plans a refactoring: `gocouple analyze --format markdown ./... > architecture.md`.
- Use the gate as the definition of done: capture `gocouple analyze --format json --out baseline.json ./...` first, then require `gocouple check --baseline baseline.json ./...` to exit 0 after the change. Only regressions fail, so existing debt does not block the work.

## Use in CI

```yaml
- uses: actions/checkout@v4
- uses: reinaldosaraiva/gocouple@v0
  with:
    args: ./...
```

`check` prints GitHub annotations when `GITHUB_ACTIONS=true`. Inputs: `args` (must be trusted, it is word-split), `baseline`, `version` (pin a tag such as `v0.1.0` for reproducible builds; the default is `latest`), `working-directory`. Thresholds, exit codes and a plain workflow are in [docs/ci.md](docs/ci.md).

## Configuration

`.gocouple.yaml` in the module root (or `--config`); flags override the file, the file overrides defaults. See [docs/metrics.md](docs/metrics.md#configuration) for every key.

## Verifying releases

Release archives are listed in `checksums.txt`, which is signed with keyless cosign, and every archive has an SBOM (Syft). Build provenance is attested with GitHub Artifact Attestations.

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/reinaldosaraiva/gocouple/\.github/workflows/release\.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --ignore-missing checksums.txt
gh attestation verify gocouple_<version>_<os>_<arch>.tar.gz --repo reinaldosaraiva/gocouple
```

## How it compares

- [spm-go](https://github.com/fdaines/spm-go) counts structs and interfaces as abstractions; `gocouple` counts only interfaces (constraint-only interfaces excluded), which is closer to the original definition.
- [Go Architect](https://go-architect.github.io/) offers an interactive dependency graph and an A x I chart; `gocouple` adds history over git, a CI gate with baselines, and diagnostics such as `wasted-abstraction`.
- [go-coupling](https://pkg.go.dev/github.com/richardwooding/go-coupling) reports Ca, Ce and I with cycles but no abstractness.
- Architecture rule linters such as [go-arch-lint](https://github.com/fe3dback/go-arch-lint) and [arch-go](https://github.com/arch-go/arch-go) enforce layer rules; `gocouple` measures shape and trend and does not enforce layers.

## Known limitations

Package granularity (not file), no detection of reflection or `any`-based injection, generic and empty interfaces skipped by `wasted-abstraction`, only the main module is classified (no `go.work` merging), and every dependency-free package without interfaces lands in the pain zone by definition of the metric, which is why `pain-zone` requires a minimum number of dependents. Details in [docs/metrics.md](docs/metrics.md#known-limitations).

## Roadmap

- Volatility from the git history combined with coupling (balanced coupling), to prioritize hotspots.
- Layer and allowlist rules, or an integration with existing architecture linters.
- Source locations in diagnostics for precise annotations.

## License

MIT, see [LICENSE](LICENSE).
