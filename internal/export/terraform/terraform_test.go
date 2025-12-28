package terraform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

func TestWriteTerraformExport(t *testing.T) {
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
	if filepath.Dir(path) != dir {
		t.Fatalf("expected output in temp dir, got %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "google_monitoring_service") {
		t.Fatalf("expected monitoring service resource in output")
	}
	if !strings.Contains(text, "google_monitoring_slo") {
		t.Fatalf("expected monitoring slo resource in output")
	}
	if !strings.Contains(text, "google_monitoring_alert_policy") {
		t.Fatalf("expected monitoring alert policy resource in output")
	}
	if !strings.Contains(text, "google_monitoring_dashboard") {
		t.Fatalf("expected monitoring dashboard resource in output")
	}
}
