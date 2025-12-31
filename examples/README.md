# Examples

- `examples/slo.yaml`: canonical ServiceSLO spec for Cloud Run.
- `examples/analyze/sample/`: sample analyze outputs (`summary.*`, `sources.json`).

Outputs from commands:

- `margin import --out out/import/...`: draft specs from existing Monitoring SLOs.
- `margin report --out out/report`: aggregated summaries from multiple analyze runs.
- Terraform export: `margin export terraform --out out/terraform` or `--module` for a module scaffold.
- Monitoring JSON export: `margin export monitoring-json --out out/monitoring-json`.
