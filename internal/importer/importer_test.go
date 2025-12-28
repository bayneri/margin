package importer

import (
	"testing"
	"time"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestParseFilter(t *testing.T) {
	filter := `metric.type="run.googleapis.com/request_count" AND resource.type="cloud_run_revision" AND metric.label.response_code="200"`
	metric, resource, extra := parseFilter(filter)
	if metric != "run.googleapis.com/request_count" {
		t.Fatalf("metric parse mismatch: %q", metric)
	}
	if resource != "cloud_run_revision" {
		t.Fatalf("resource parse mismatch: %q", resource)
	}
	if extra != `metric.label.response_code="200"` {
		t.Fatalf("extra parse mismatch: %q", extra)
	}
}

func TestDurationToWindow(t *testing.T) {
	window, err := durationToWindow(durationpb.New(30 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if window != "30d" {
		t.Fatalf("expected 30d, got %q", window)
	}
}

func TestInferServiceType(t *testing.T) {
	slo := &monitoringpb.ServiceLevelObjective{
		Name:        "projects/demo/services/checkout-api/serviceLevelObjectives/availability",
		DisplayName: "availability",
		Goal:        0.999,
		ServiceLevelIndicator: &monitoringpb.ServiceLevelIndicator{
			Type: &monitoringpb.ServiceLevelIndicator_RequestBased{
				RequestBased: &monitoringpb.RequestBasedSli{
					Method: &monitoringpb.RequestBasedSli_GoodTotalRatio{
						GoodTotalRatio: &monitoringpb.TimeSeriesRatio{
							GoodServiceFilter:  `metric.type="run.googleapis.com/request_count" AND resource.type="cloud_run_revision" AND metric.label.response_code="200"`,
							TotalServiceFilter: `metric.type="run.googleapis.com/request_count" AND resource.type="cloud_run_revision"`,
						},
					},
				},
			},
		},
		Period: &monitoringpb.ServiceLevelObjective_RollingPeriod{
			RollingPeriod: durationpb.New(30 * 24 * time.Hour),
		},
	}

	if got := inferServiceType([]*monitoringpb.ServiceLevelObjective{slo}); got != "cloud-run" {
		t.Fatalf("expected cloud-run, got %q", got)
	}
}
