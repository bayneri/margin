package spec

import "testing"

func TestValidatePeriod(t *testing.T) {
	cases := []struct {
		name   string
		period string
		window string
		wantOK bool
	}{
		{"default", "", "30d", true},
		{"rolling", "rolling", "30d", true},
		{"rolling-invalid", "rolling", "", false},
		{"calendar-ok", "calendar", "1w", true},
		{"calendar-invalid", "calendar", "7d", false},
		{"bad-period", "foo", "30d", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePeriod(tc.period, tc.window)
			if tc.wantOK && got != "" {
				t.Fatalf("expected ok, got %q", got)
			}
			if !tc.wantOK && got == "" {
				t.Fatalf("expected error, got ok")
			}
		})
	}
}

func TestQualifiedFilter(t *testing.T) {
	cases := []struct {
		filter string
		want   bool
	}{
		{"metric.label.response_code = \"200\"", true},
		{"resource.type = \"cloud_run_revision\"", true},
		{"project.id = \"demo\"", true},
		{"metadata.user_labels.env = \"prod\"", true},
		{"group.id = \"abc\"", true},
		{"response_code = \"200\"", false},
		{"", false},
	}

	for _, tc := range cases {
		got := qualifiedFilter(tc.filter)
		if got != tc.want {
			t.Fatalf("qualifiedFilter(%q)=%v, want %v", tc.filter, got, tc.want)
		}
	}
}

func TestValidateLatencyThreshold(t *testing.T) {
	template := ServiceTemplate{
		Name: "cloud-run",
		Metrics: map[string]MetricTemplate{
			"run.googleapis.com/request_latencies": {Name: "run.googleapis.com/request_latencies"},
		},
	}
	sli := SLI{
		Type:      "latency",
		Metric:    "run.googleapis.com/request_latencies",
		Threshold: "bad",
	}
	errs := validateSLI(sli, template)
	if len(errs) == 0 {
		t.Fatalf("expected error for bad threshold")
	}

	sli.Threshold = "500ms"
	errs = validateSLI(sli, template)
	if len(errs) != 0 {
		t.Fatalf("expected ok threshold, got %v", errs)
	}
}

func TestValidateSLOAlerting(t *testing.T) {
	cases := []struct {
		name     string
		alerting SLOAlerting
		wantOK   bool
	}{
		{"empty", SLOAlerting{}, true},
		{"fast-ok", SLOAlerting{Fast: &AlertOverride{Windows: []string{"5m", "1h"}, BurnRate: 10}}, true},
		{"slow-bad-windows", SLOAlerting{Slow: &AlertOverride{Windows: []string{"5m"}}}, false},
		{"fast-bad-window", SLOAlerting{Fast: &AlertOverride{Windows: []string{"5m", "bad"}}}, false},
		{"fast-negative-burn", SLOAlerting{Fast: &AlertOverride{BurnRate: -1}}, false},
		{"fast-low-burn", SLOAlerting{Fast: &AlertOverride{Windows: []string{"5m", "1h"}, BurnRate: 0.5}}, false},
		{"fast-same-window", SLOAlerting{Fast: &AlertOverride{Windows: []string{"5m", "5m"}, BurnRate: 2}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateSLOAlerting(tc.alerting)
			if tc.wantOK && got != "" {
				t.Fatalf("expected ok, got %q", got)
			}
			if !tc.wantOK && got == "" {
				t.Fatalf("expected error, got ok")
			}
		})
	}
}
