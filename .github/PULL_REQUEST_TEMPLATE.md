## What and why

<!-- What does this change and why is it needed? -->

## Checks

- [ ] `go build ./... && go vet ./... && go test -race ./...`
- [ ] `golangci-lint run` and `gofmt -l .` are clean
- [ ] `go run ./cmd/gocouple check ./...` passes
- [ ] Tests and, if outputs changed, golden files were added or updated
- [ ] Commits follow Conventional Commits
