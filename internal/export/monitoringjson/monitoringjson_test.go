package monitoringjson

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

func TestWriteMonitoringJSON(t *testing.T) {
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

	plan := planner.Build(specDoc, planner.Options{})
	template, err := spec.TemplateForService(specDoc.Metadata.Service)
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	dir := t.TempDir()
	path, err := Write(plan, template, dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["service"] == nil {
		t.Fatalf("expected service payload")
	}
	if payload["slos"] == nil {
		t.Fatalf("expected slos payload")
	}
	if payload["alertPolicies"] == nil {
		t.Fatalf("expected alertPolicies payload")
	}
	if payload["dashboard"] == nil {
		t.Fatalf("expected dashboard payload")
	}
}
