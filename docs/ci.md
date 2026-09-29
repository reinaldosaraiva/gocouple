# Using gocouple in CI

`gocouple check` returns:

| Exit code | Meaning |
|-----------|---------|
| 0 | all thresholds hold |
| 1 | at least one violation |
| 2 | usage or execution error (bad flag, load failure, unreadable baseline) |

With `GITHUB_ACTIONS=true` every violation is also printed as a workflow command (`::error title=...::message`).

## Thresholds

| Flag | Config key | Default | Rule |
|------|------------|---------|------|
| `--max-distance` | `check.max_distance` | 0.7 | no relevant package (uselessness zone, or pain zone with Ca >= `min_ca_for_pain`) is farther from the main sequence |
| `--max-pain-packages` | `check.max_pain_packages` | 0 | number of `pain-zone` diagnostics |
| `--fail-on-cycles` | `check.fail_on_cycles` | true | any `dependency-cycle` fails |
| `--fail-on` | | error | lowest diagnostic severity that fails |
| `--baseline`, `--tolerance` | | 0.02 | fail only on a new pain package, a new cycle, or an average distance increase above the tolerance |

## GitHub Action

```yaml
name: architecture
on: [pull_request]
jobs:
  coupling:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: reinaldosaraiva/gocouple@v0
        with:
          args: ./...
```

Inputs: `args` (passed to `gocouple check` after word splitting, so it must come from a trusted source; default `./...`), `baseline` (path to an `analysis.json`), `version` (default `latest`; pin a tag for reproducible builds), `working-directory` (default `.`).

## Plain workflow

```yaml
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go install github.com/reinaldosaraiva/gocouple/cmd/gocouple@latest
      - run: gocouple check ./...
```

## Baseline mode

Commit `analysis.json` (from `gocouple analyze --format json --out analysis.json ./...`) and run `gocouple check --baseline analysis.json ./...`. Only a new pain-zone package, a new cycle, or an average distance increase above `--tolerance` fails.
