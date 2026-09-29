# Examples

- `report.html`: a report with a commit slider, generated from a small synthetic history: a service gets an over-connected package and an import cycle, then is refactored. Open it in a browser; it needs no network.
- `orders-legacy.html`: the report of `testdata/orders-legacy`, the deliberately problematic order system used in the README.

Regenerate `orders-legacy.html` with:

```bash
gocouple analyze --dir testdata/orders-legacy --format json --out analysis.json ./...
gocouple report --snapshot analysis.json --out examples/orders-legacy.html
```

`report.html` is the golden file of the report tests (`internal/report/testdata/history.golden.html`); regenerate it with `go test ./internal/report -update` and copy it here.
