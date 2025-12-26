# Alerting model

## Burn-rate math

A burn rate is how fast an SLO consumes its error budget relative to its target window.
For an SLO with objective 99.9% over 30 days, a 14.4x burn rate means the budget is being spent
14.4 times faster than expected, which would exhaust the budget in roughly 2.1 days.

Burn rate is computed as:

```
(error_rate / (1 - objective))
```

Where `error_rate` is the observed fraction of bad requests over the alert window.

## Multi-window alerts

`margin` uses multi-window, multi-burn alerts to reduce noise while still paging quickly
when error budgets are being rapidly consumed.

Default windows and burn rates:

- Fast burn: 5m / 1h at 14.4x (page)
- Slow burn: 30m / 6h at 6x (ticket)

The fast window catches rapid regressions, while the slow window confirms the issue
is sustained before paging humans.

## Paging philosophy

- Page only when the error budget is burning fast enough to require immediate action.
- Ticket for slower burns to preserve the budget and force prioritization.
- Avoid threshold-based alerts for symptoms already captured by SLOs.
