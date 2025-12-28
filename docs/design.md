# Design

## Architecture

`margin` is a single-binary CLI that:

1. Parses a v1 `ServiceSLO` spec
2. Validates objective math and template constraints
3. Plans the concrete Monitoring resources (SLOs, alerts, dashboards)
4. Applies them idempotently via Google Cloud Monitoring APIs
5. Exports equivalent Terraform/JSON payloads
6. Imports existing Monitoring SLOs into a draft spec (read-only)
7. Aggregates analyze outputs into a combined report (read-only)

The code is organized by responsibility:

- `internal/spec` handles YAML parsing and validation
- `internal/planner` translates specs into concrete resources
- `internal/alerting` owns burn-rate logic and explainers
- `internal/monitoring` wraps Monitoring API calls
- `internal/analyze` reads Monitoring data and produces incident reports
- `internal/report` renders Markdown/JSON artifacts
- `internal/export` emits Terraform/Monitoring JSON (read-only)
- `internal/importer` ingests existing SLOs into a draft spec (read-only)
- `internal/report` also aggregates analyze outputs into combined reports (read-only)
- `internal/dashboard` defines dashboard layout primitives

## Why a CLI

- It keeps operational workflows explicit and reviewable in code reviews.
- It integrates cleanly with existing release pipelines.
- It avoids introducing a long-running control plane for a narrow problem.

## Why API-driven instead of Terraform-only

Terraform can create Monitoring resources, but:

- It struggles to encode SLO-specific math and guardrails without custom providers.
- It makes it harder to generate derived resources (alerts, dashboards) with consistent opinionated defaults.
- It doesn't provide an interactive `explain` workflow for SRE education.

`margin` can still coexist with Terraform: the CLI can be run in a pipeline, and later export support can bridge the two.

## Trade-offs and alternatives

- `margin` focuses on a narrow set of GCP services to ensure safe defaults.
- It uses opinionated templates to prevent arbitrary metric misuse.
- For teams that prefer full flexibility, direct Monitoring API usage or Terraform modules remain valid alternatives.
