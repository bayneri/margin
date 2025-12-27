# Incident window analysis

- Service: checkout
- Project: demo
- Window: 2025-01-01T10:00:00Z to 2025-01-01T11:00:00Z
- Duration: 1h0m0s

| SLO | Goal | Compliance | Bad fraction | Allowed bad | Budget consumed | Status |
| --- | --- | --- | --- | --- | --- | --- |
| availability | 0.9990 | 0.9950 | 0.0050 | 0.0010 | 500.00% | ok |

## How computed

Formula: allowedBad = 1 - goal; bad = 1 - compliance; consumedPercent = (bad / allowedBad) * 100
