# Report

`margin report` aggregates multiple `margin analyze` outputs into a combined view.

Inputs:

- Comma-separated `--inputs` pointing to analyze `summary.json` files.

Outputs:

- `summary.json` (aggregate status + per-service SLOs)
- `summary.md` (Markdown summary)

Exit codes:

- 0 on success
- 2 on partial (any errors/unsupported SLOs in inputs)

Example:

```bash
./margin report --inputs out/a/summary.json,out/b/summary.json --out out/report
```

Aggregation rules:
- Overall status is the worst across all services/SLOs (breach > partial/error > ok).
- Partial inputs produce warnings and exit code 2.
- SLO rows are merged per service; errors from inputs are preserved.
