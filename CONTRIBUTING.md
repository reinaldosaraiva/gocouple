# Contributing

Thanks for helping. This project measures architecture, so it holds itself to the same standard.

## Setup

Go 1.26 or newer, `git`, and [golangci-lint](https://golangci-lint.run/) v2 on the PATH.

```bash
make build   # bin/gocouple
make test    # go test -race ./...
make lint    # golangci-lint run
make cover   # coverage summary
make fmt     # gofmt and goimports
```

## Before you open a pull request

- `go build ./... && go vet ./... && go test -race ./...` pass and `gofmt -l .` prints nothing.
- `golangci-lint run` reports no issues.
- `go run ./cmd/gocouple check ./...` passes (the repository dogfoods itself).
- New behavior has tests; table-driven where it makes sense. Outputs that must stay stable have golden files under `testdata/golden` (regenerate with `go test ./cmd/... -update` and review the diff).
- Commits follow Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`, `ci:`), one logical change per commit, subject up to 72 characters.

## Design rules

- `internal/metrics` and `internal/graph` are pure: structs in, structs out, no I/O.
- Adding a diagnostic means adding one file in `internal/diagnose` and one entry in `Rules()`; no other rule changes.
- Output is deterministic: sort everything by name so JSON and CSV are byte-stable.
- Dependencies are kept minimal: `golang.org/x/tools/go/packages`, `cobra`, `yaml.v3`; tests use `testing` and `go-cmp`.
- Do not use a git library; `history` talks to the `git` binary and must never touch the user's working tree.

## Reporting problems

Use the issue templates. For security problems see [SECURITY.md](SECURITY.md).
