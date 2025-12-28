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

## Per-SLO overrides

You can override burn-rate windows and burn rate per SLO:

```yaml
slos:
  - name: availability
    objective: 99.9
    window: 30d
    sli:
      type: request-based
      good:
        metric: run.googleapis.com/request_count
        filter: 'metric.label.response_code < 500'
      total:
        metric: run.googleapis.com/request_count
    alerting:
      fast:
        windows: ["2m", "30m"]
        burnRate: 20
      slow:
        windows: ["1h", "12h"]
        burnRate: 3
```

Only the fields you set are overridden; missing values keep defaults.

Validation:

- Alert override windows must be two ordered values (short, long) and at least 1m, burnRate >= 1.
- Filters must include a `resource.type` matching the service template (e.g., `cloud_run_revision`, `https_lb_rule`).

## Paging philosophy

- Page only when the error budget is burning fast enough to require immediate action.
- Ticket for slower burns to preserve the budget and force prioritization.
- Avoid threshold-based alerts for symptoms already captured by SLOs.

## Burn-rate alert implementation

Cloud Monitoring requires a resource type in burn-rate alert filters, and the burn-rate
series can lag SLO creation. `margin` uses the `select_slo_burn_rate` filter form to
reference the SLO directly instead of relying on precomputed time series.

Example filter (short window):

```
select_slo_burn_rate("projects/PROJECT/services/SERVICE_ID/serviceLevelObjectives/SLO_ID", "60m")
```

If your environment requires a specific resource type for burn-rate alerts, set it in
the spec:

```yaml
alerting:
  burnRateResourceType: global
```
