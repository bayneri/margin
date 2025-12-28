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

func TestBuildPerSLOAlertOverrides(t *testing.T) {
	specDoc := spec.Spec{
		APIVersion: spec.APIVersionV1,
		Kind:       spec.KindServiceSLO,
		Metadata: spec.Metadata{
			Name:    "checkout-api",
			Service: "cloud-run",
			Project: "demo",
		},
		SLOs: []spec.SLO{
			{
				Name:      "availability",
				Objective: 99.9,
				Window:    "30d",
				SLI: spec.SLI{
					Type:  "request-based",
					Good:  &spec.MetricDef{Metric: "run.googleapis.com/request_count", Filter: "metric.label.response_code = \"200\""},
					Total: &spec.MetricDef{Metric: "run.googleapis.com/request_count"},
				},
				Alerting: spec.SLOAlerting{
					Fast: &spec.AlertOverride{Windows: []string{"2m", "30m"}, BurnRate: 20},
					Slow: &spec.AlertOverride{Windows: []string{"1h", "12h"}, BurnRate: 3},
				},
			},
			{
				Name:      "latency",
				Objective: 99,
				Window:    "30d",
				SLI: spec.SLI{
					Type:      "latency",
					Metric:    "run.googleapis.com/request_latencies",
					Threshold: "500ms",
				},
			},
		},
	}

	plan := Build(specDoc, Options{})
	for _, alert := range plan.Alerts {
		switch alert.SLOName {
		case "availability":
			if alert.Type == "fast-burn" {
				if !equalWindows(alert.Windows, []string{"2m", "30m"}) {
					t.Fatalf("expected availability fast windows override, got %v", alert.Windows)
				}
				if alert.BurnRate != 20 {
					t.Fatalf("expected availability fast burnRate 20, got %v", alert.BurnRate)
				}
			}
			if alert.Type == "slow-burn" {
				if !equalWindows(alert.Windows, []string{"1h", "12h"}) {
					t.Fatalf("expected availability slow windows override, got %v", alert.Windows)
				}
				if alert.BurnRate != 3 {
					t.Fatalf("expected availability slow burnRate 3, got %v", alert.BurnRate)
				}
			}
		case "latency":
			if alert.Type == "fast-burn" {
				if !equalWindows(alert.Windows, []string{"5m", "1h"}) {
					t.Fatalf("expected latency fast default windows, got %v", alert.Windows)
				}
				if alert.BurnRate != 14.4 {
					t.Fatalf("expected latency fast default burnRate 14.4, got %v", alert.BurnRate)
				}
			}
			if alert.Type == "slow-burn" {
				if !equalWindows(alert.Windows, []string{"30m", "6h"}) {
					t.Fatalf("expected latency slow default windows, got %v", alert.Windows)
				}
				if alert.BurnRate != 6 {
					t.Fatalf("expected latency slow default burnRate 6, got %v", alert.BurnRate)
				}
			}
		}
	}
}

func equalWindows(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
