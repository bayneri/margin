# Import

`margin import` reads existing Monitoring SLOs and emits a draft `ServiceSLO` spec.

Supported conversions:

- Request-based SLIs (good/total) -> request-based
- Request-based with `bad_service_filter` -> request-based (good = total minus bad)
- Request-based `distribution_cut` (latency) -> latency SLI
- Windows-based SLIs using request-based or distribution-cut performance -> converted to request-based or latency
- Basic SLIs:
  - Availability -> request-based using template request metrics
  - Latency -> latency threshold using template latency metrics

Unsupported/partial:
- Basic SLIs without explicit metrics and no template mapping
- Composite/window criteria beyond good/total or distribution cut

Warnings are emitted for skipped/partial SLOs; the generated spec remains editable.

Examples:

```bash
./margin import --project my-gcp-project --service checkout-api --out out/import/checkout-api.yaml
```
