# Incident window analysis

- Service: checkout-api
- Project: my-gcp-project
- Window: 2025-12-01T10:12:00Z to 2025-12-01T11:04:00Z
- Duration: 52m0s
- Status: ok

| SLO | Goal | Compliance | Bad fraction | Allowed bad | Budget consumed | Status |
| --- | --- | --- | --- | --- | --- | --- |
| availability | 0.9990 | 0.9950 | 0.0050 | 0.0010 | 500.00% | ok |
| latency | 0.9900 | 0.9870 | 0.0130 | 0.0100 | 130.00% | ok |

## How computed

Formula: allowedBad = 1 - goal; bad = 1 - compliance; consumedPercent = (bad / allowedBad) * 100
