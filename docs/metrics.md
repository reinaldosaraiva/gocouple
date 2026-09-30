# Metrics and diagnostics

Unit of analysis: a Go package (import path) of the analyzed module. Standard library imports never count. Third-party imports count toward Ce only with `--include-external` (or `include_external: true`). Test packages are ignored unless `--include-tests` is set.

## Metrics

| Metric | Definition |
|--------|------------|
| Ca | distinct internal packages that import the package |
| Ce | distinct packages the package imports (internal, plus external with `--include-external`) |
| I | `Ce / (Ca + Ce)`; 0 when `Ca + Ce == 0` |
| Nc | named types declared in the package (aliases excluded) |
| Na | interfaces among them; constraint-only interfaces (type sets) do not count |
| A | `Na / Nc`; 0 when `Nc == 0` |
| D | `abs(A + I - 1)` |

`--exported-only` counts only exported types in Nc and Na. Values are computed in full precision and rounded to four decimals only when serialized.

### Zones (`distance_threshold`, default 0.5)

| Zone | Rule |
|------|------|
| `isolated` | `Ca + Ce == 0` |
| `main_sequence` | `D <= threshold` (a package exactly at the threshold is on the main sequence) |
| `pain` | `D > threshold` and `A + I < 1`: stable and concrete |
| `uselessness` | `D > threshold` and `A + I > 1`: abstract and unstable |

Worked example: a logger package with one struct (Nc 1, Na 0), imported by 10 packages and importing none has Ca 10, Ce 0, I 0, A 0, D 1 and lands in `pain`.

## Diagnostics

| Id | Severity | Fires when | Default threshold |
|----|----------|------------|-------------------|
| `pain-zone` | warning | zone `pain` and `Ca >= min_ca_for_pain` | 3 |
| `uselessness-zone` | warning | zone `uselessness` | |
| `god-package` | warning | `Ce >= god_ce_threshold` (evidence lists the imports) | 8 |
| `dependency-cycle` | error | a strongly connected component of two or more packages (iterative Tarjan); evidence is the shortest cycle from the smallest name | |
| `concrete-hotspot` | warning | `Ca >= hotspot_ca_threshold` and `A == 0` | 5 |
| `sdp-violation` | warning | edge `p -> q` with `I(q) > I(p) + sdp_tolerance`; `main` packages and `composition_roots` are exempt as importers | tolerance 0 |
| `wasted-abstraction` | warning | for each exported interface: no package outside its own references it (`no consumers`), or more packages reference its implementations than the interface itself (`not inverted`); the second wins when both hold | |

`wasted-abstraction` details: implementations are the module's named non-interface types, or pointers to them, that satisfy the interface (`types.Implements`). A concrete reference is a use of the type name (composite literal, field, parameter) or a call to a function returning it (also inside pointers, slices, arrays, maps and channels). References from the interface's package, from the implementer's own package, from `main` packages and from composition roots do not count.

## Generated code and ignored findings

A non-test file whose header matches the Go convention `// Code generated ... DO NOT EDIT.` is generated. Interfaces declared in generated files are never analyzed by `wasted-abstraction`. A package with at least one generated file and no hand-written declaration (imports aside) is pure generated: it is still measured, and still counts in the `Ca` and `Ce` of its neighbours, but its diagnostics are suppressed and it is exempt from `check` `max_distance`. Packages carry `"generated": true` in the JSON.

An `ignore` entry silences one rule on the packages matching a glob when the finding is understood and accepted, for example a value struct read once at startup. Unlike `exclude`, the package stays in every metric, so neighbours and averages are not distorted. `reason` is mandatory, `rule` must be a known diagnostic id, and every silenced finding is listed under `suppressed` in the JSON and in a `Suppressed` section of the table and markdown output, with its reason.

```yaml
ignore:
  - rule: pain-zone
    package: "**/internal/config"
    reason: value struct read once at startup
```

For `check`, a package whose `pain-zone` or `uselessness-zone` finding is silenced also stops counting against `max_distance`.

## Volatility (opt-in)

`--volatility-since <window>` (or `volatility.since` in `.gocouple.yaml`; the flag wins, an empty flag turns it off) reads the git history once and adds two fields to every package: `churn`, the number of non-merge commits in the window that touched at least one non-test `.go` file directly in the package directory, and `volatility`, `churn` divided by the largest churn of the analysis (0 when nothing changed). The window is a duration in days (`180d`), a date (`2026-01-01`) or an RFC3339 time; it is resolved once to an absolute UTC boundary that is stored as `config.volatility_since` in the JSON. Both fields are omitted from the JSON when zero, and the `CHURN` column appears in the table, csv, markdown and report only when volatility was measured. Without the flag the output is identical to a run without the feature.

Effect on `pain-zone` (balanced coupling: a rigid dependency is acceptable when the package does not change):

- churn 0 in the window: the finding moves to `Suppressed` with the reason `stable in window (no commit since <YYYY-MM-DD>)`; it is listed, never dropped;
- churn above 0: the finding keeps its severity and the message adds `changed N times since <YYYY-MM-DD>`, and the evidence carries `churn`;
- an `ignore` reason and the generated-code rule take precedence over the dormant rule.

Environment problems are warnings, not errors: outside a git work tree, in a shallow clone, or when git cannot tell, the analysis runs without volatility and reports why. A failing `git log` or an invalid window is an error. `history` does not compute volatility (it prints a note when the key is set).

Limitations: the window filters on the committer date (`git log --since-as-filter`), so a rebased or cherry-picked commit is dated by its rewrite; renames count at the destination package only; generated files count as changes; merge commits are excluded; the unit is the package, not the file; a repository younger than the window makes every package look active, so choose a window shorter than the project's age.

## Configuration

`.gocouple.yaml` (discovered in `--dir`, or `--config`); flags win over the file, the file wins over defaults; unknown keys are errors.

```yaml
distance_threshold: 0.5
min_ca_for_pain: 3
god_ce_threshold: 8
hotspot_ca_threshold: 5
sdp_tolerance: 0.0
exported_only: false
include_external: false
exclude: ["**/mocks/**"]          # dropped before any metric is computed
composition_roots: ["**/cmd/**", "**/internal/wire/**"]
ignore:                           # keeps the package measured, silences one rule
  - rule: pain-zone
    package: "**/internal/config"
    reason: value struct read once at startup
check:
  max_distance: 0.7
  max_pain_packages: 0
  fail_on_cycles: true
volatility:
  since: 180d                     # off when absent; a duration, a date or RFC3339
```

Globs match import paths; `**` spans segments, `*` stays inside one.

## check

See [ci.md](ci.md). `max_distance` is a per-package ceiling applied to packages in the uselessness zone and to pain-zone packages with at least `min_ca_for_pain` dependents; `max_pain_packages` counts `pain-zone` diagnostics.

## Known limitations

- Granularity is the package, not the file; a package with one large file and one small file is one unit.
- Reflection, `any`-based injection and code generation that hides imports are invisible.
- Generated code is recognized only by the standard header; generators that omit it are not detected, and `ignore` is the fallback.
- A file that carries the generated header and also hand-written declarations counts as generated, as gopls and golangci-lint treat it; move the hand-written code to another file to have it diagnosed.
- Without `--volatility-since`, `pain-zone` cannot tell a stable package that never changes from one that changes often; see [Volatility](#volatility-opt-in) for the limits of the measurement.
- Generic interfaces, empty interfaces and constraint interfaces are skipped by `wasted-abstraction`.
- Interface matching is structural: unrelated types with an identical method set count as implementers.
- Only the main module is classified; nested modules and `go.work` workspaces are not merged, and imports of other workspace modules count as external.
- `--include-tests` merges in-package test files into the package (their types and imports count); external test packages (`p_test`) are separate packages.
- Standard library detection is the first import path element without a dot.
- The A x I definition puts every dependency-free package without interfaces (a leaf library used by one caller) in the `pain` zone; that is why `pain-zone` requires `Ca >= min_ca_for_pain` and why `check` counts diagnostics rather than raw zones.
- `history` analyzes each commit in a detached worktree, so cost grows with the number of commits; results are cached by commit and configuration.
- Diagnostics carry a package, not a source location, so GitHub annotations have a title and message but no file anchor.
- The dependency graph in the HTML report is capped at 60 packages and 400 edges; use `--format dot` or `--format mermaid` beyond that.
