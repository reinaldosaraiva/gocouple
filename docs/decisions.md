# Design decisions

Decisions taken while building `gocouple`, with the reasoning and the measurements behind them.

## Toolchain and module

- Module path: `github.com/reinaldosaraiva/gocouple`.
- Go floor: `go 1.26`. The original plan asked for `go 1.23`, but `golang.org/x/tools` v0.50.0 requires go 1.26.0 (the last line supporting 1.23 is v0.36.x). The CI matrix runs `stable` and `oldstable`, which resolve to Go 1.27.1 and 1.26.8 at the time of writing, both at or above the floor.

## Loader semantics

- Internal dependency: import path equal to or under the main module path. Standard library: first path element without a dot. Anything else is external and counts toward Ce only with `--include-external`.
- `--include-tests`: the test-augmented variant of a package replaces the plain one, so test-file imports and types are counted; external test packages (`p_test`) are separate packages; generated `*.test` binaries are skipped.
- A package importing itself is dropped from the graph (it is not a cycle of two or more packages).
- Packages are loaded with `packages.NeedDeps` so that every package shares one type universe, which `wasted-abstraction` needs to compare types across packages.

## Dogfooding

`gocouple check ./...` must pass on this repository. Pure-data packages are stable and concrete by design, so the metrics place them in the `pain` zone. Decision: `check --max-pain-packages` counts `pain-zone` diagnostics (which require `Ca >= min_ca_for_pain`) instead of raw zones, and this repository's `.gocouple.yaml` excludes the pure-data packages `internal/model` and `internal/config`.

## history

- Sampling: commits are listed newest first, one of every `--every` is kept starting at the newest, at most `--max` are kept, and the result is reversed to oldest first.
- Commit dates are normalized to UTC (RFC 3339) so JSON and CSV do not depend on the committer's timezone.
- Cache lives in `<repo>/.gocouple/cache/<sha>-<key>.json`; the key hashes tool version, effective analysis config, load flags, patterns and the module directory relative to the repository. This repository ignores `.gocouple/`; other repositories should too.
- Failed commits are recorded as `{commit, error}` entries and are not cached.
- The analysis pipeline lives in `internal/analysis`, shared by `analyze` and `history`.
- The first SIGINT or SIGTERM cancels the run and removes every worktree; a second signal terminates immediately.

## check semantics

- `max_distance` is a per-package ceiling applied to packages that matter: those in the uselessness zone and those in the pain zone with at least `min_ca_for_pain` dependents. Reading it as a ceiling on the average distance was rejected: on this repository many small leaf packages (Ca 1, Ce 0, no interfaces) are stable and concrete, so the average distance is 0.72 even though nothing is painful. The original plan did not define the meaning; revisit if users expect another one.
- `max_pain_packages` counts `pain-zone` diagnostics, not raw zones.
- `--baseline` mode ignores absolute thresholds and fails only on a new `pain-zone` package, a new cycle, or an average distance increase above `--tolerance`.
- Generated code and `ignore`: `pain-zone` fired on protobuf stubs and on value structs, where its advice (introduce an interface) does not apply. Decision: recognize generated code by the standard header (pure generated packages keep their metrics but lose their diagnostics; generated interfaces skip `wasted-abstraction`), and add `ignore` entries with a mandatory reason that keep the package measured, unlike `exclude`. Suppressed findings are listed, never dropped silently.
- Volatility (P003, supersedes the roadmap-only decision of contract Q21): opt-in, one `git log --since-as-filter` per analysis from the module root, churn per package and a ratio to the largest churn. A `pain-zone` package with no commit in the window is suppressed with a reason instead of dropped, so the finding stays auditable, and it no longer counts in `check` or in a `--baseline` regression. The git runner lives in the neutral package `internal/gitrun` so that `analysis` and `volatility` do not turn `internal/history` into a pain-zone package themselves. Measured on the mgc-connect components and on this repository (all younger than two months), no package was dormant in any window, so the feature changed no verdict there and the churn column became the useful output.
- Known limitation: the A x I metrics classify every dependency-free package with no interfaces as `pain`; `min_ca_for_pain` is the intended guard for that case.

## Real-project validation

Read-only shallow clones (`--depth 60`) in a temporary directory outside the repository, never committed; measured on macOS arm64, Go 1.26.

| Project | Packages | `analyze` | `check` | `history --max 20 --workers 4` |
|---------|----------|-----------|---------|--------------------------------|
| spf13/cobra | 2 | 1.4s (cold) | | |
| go-chi/chi | 2 | 0.4s | | |
| caddyserver/caddy | 48 | 17.8s (first run, cold module cache), 1.3s warm | exit 1, 4 violations | 20 analyzed, 0 failed, 24.7s; 20 cached, 0.08s on the second run |

- No crash or hang; `git worktree list` showed only the main worktree afterwards and no `gocouple-wt-*` temp directories remained.
- Caddy: 14 of 48 packages land in the `pain` zone (small leaf helpers such as `notify` and `caddyconfig/warning`); the tool reports 29 `wasted-abstraction` findings, mostly interfaces implemented and consumed inside one package. Both follow from the definitions documented in `metrics.md`; they are not loader problems.
- Benchmarks (`b.Loop()`, warm module cache): `analysis.Run` on `orders-legacy` 204 ms and 2.3M allocs; on this repository 325 ms and 5.4M allocs; `metrics.Analyze` on 1,000 packages 2.3 ms. The cost is dominated by type-checking standard library sources under `packages.NeedDeps`. Decision: keep `NeedDeps`; no cheaper mode until a measured need appears.
- Test suite: the `cmd` package tests dropped from about 36s to 21s by running the golden subtests in parallel.

## Release tooling

- GoReleaser is validated with `go run github.com/goreleaser/goreleaser/v2@latest check` and a local `release --snapshot --clean --skip=sign,sbom`; signing and SBOM generation need `cosign` and `syft`, which the release workflow installs.
- `.goreleaser.yaml` sets `release.github` explicitly so it also works before a git remote exists. A local snapshot needs `GORELEASER_FORCE_TOKEN=github` and a placeholder `GITHUB_TOKEN` when other forge tokens are present in the environment.
- Third-party actions are pinned by commit SHA with the tag in a trailing comment.
- Major tag `v0` moves to the latest `v0.x.y` release after each release (`git tag -f v0 vX.Y.Z` and a force push of the tag), so `uses: reinaldosaraiva/gocouple@v0` follows the current minor. The release workflow only fires on full semver tags, so moving `v0` never re-runs it.
