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
