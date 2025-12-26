# Analyze

`margin analyze` computes an incident window impact report for SLOs in Cloud Monitoring.
It is read-only and designed for postmortems.

## Supported SLO types (v0)

- Request-based SLOs using good/total ratio
- Request-based latency SLOs using distribution cut

All other SLO shapes are marked as partial with an explanation.

## Math

For each SLO:

- `allowedBad = 1 - goal`
- `bad = 1 - compliance`
- `consumedPercent = (bad / allowedBad) * 100`

This is a window-local ratio. It is not the same as remaining budget over the full rolling period.

## Flags

- `--start` and `--end` (RFC3339) or `--last` (duration)
- `--out` output directory
- `--format md,json`
- `--explain` include formulas
- `--timezone` for report timestamps
- `--max-slos` limit SLOs analyzed
- `--only` filter by regex
- `--fail-on-partial` exit non-zero on partial results

## Caveats

- Compliance is fetched via Cloud Monitoring time series; missing data yields partial output.
- Calendar-period SLOs are supported, but still reported with window-local math.
