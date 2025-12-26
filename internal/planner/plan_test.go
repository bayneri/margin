package planner

import (
	"testing"

	"github.com/bayneri/margin/internal/spec"
)

func TestBuildDefaultsBurnRateResourceType(t *testing.T) {
	specDoc := spec.Spec{
		APIVersion: spec.APIVersionV1,
		Kind:       spec.KindServiceSLO,
		Metadata: spec.Metadata{
			Name:    "checkout-api",
			Service: "cloud-run",
			Project: "demo",
		},
		SLOs: []spec.SLO{{
			Name:      "availability",
			Objective: 99.9,
			Window:    "30d",
			SLI: spec.SLI{
				Type:  "request-based",
				Good:  &spec.MetricDef{Metric: "run.googleapis.com/request_count", Filter: "metric.label.response_code = \"200\""},
				Total: &spec.MetricDef{Metric: "run.googleapis.com/request_count"},
			},
		}},
	}

	plan := Build(specDoc, Options{})
	if plan.BurnRateResourceType != "global" {
		t.Fatalf("expected default burnRateResourceType global, got %q", plan.BurnRateResourceType)
	}
	for _, alert := range plan.Alerts {
		if alert.BurnRateResourceType != "global" {
			t.Fatalf("expected alert burnRateResourceType global, got %q", alert.BurnRateResourceType)
		}
	}
}

func TestBuildOverridesBurnRateResourceType(t *testing.T) {
	specDoc := spec.Spec{
		APIVersion: spec.APIVersionV1,
		Kind:       spec.KindServiceSLO,
		Metadata: spec.Metadata{
			Name:    "checkout-api",
			Service: "cloud-run",
			Project: "demo",
		},
		Alerting: spec.Alerting{BurnRateResourceType: "custom_resource"},
		SLOs: []spec.SLO{{
			Name:      "availability",
			Objective: 99.9,
			Window:    "30d",
			SLI: spec.SLI{
				Type:  "request-based",
				Good:  &spec.MetricDef{Metric: "run.googleapis.com/request_count", Filter: "metric.label.response_code = \"200\""},
				Total: &spec.MetricDef{Metric: "run.googleapis.com/request_count"},
			},
		}},
	}

	plan := Build(specDoc, Options{})
	if plan.BurnRateResourceType != "custom_resource" {
		t.Fatalf("expected burnRateResourceType custom_resource, got %q", plan.BurnRateResourceType)
	}
	for _, alert := range plan.Alerts {
		if alert.BurnRateResourceType != "custom_resource" {
			t.Fatalf("expected alert burnRateResourceType custom_resource, got %q", alert.BurnRateResourceType)
		}
	}
}
