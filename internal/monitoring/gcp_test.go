package monitoring

import (
	"testing"

	"google.golang.org/genproto/googleapis/type/calendarperiod"
)

func TestBuildPeriodRolling(t *testing.T) {
	kind, rolling, calendar, err := buildPeriod("rolling", "30d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "rolling" {
		t.Fatalf("expected rolling, got %q", kind)
	}
	if rolling == 0 {
		t.Fatalf("expected rolling duration")
	}
	if calendar != 0 {
		t.Fatalf("expected no calendar period")
	}
}

func TestBuildPeriodCalendar(t *testing.T) {
	kind, rolling, calendar, err := buildPeriod("calendar", "1w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "calendar" {
		t.Fatalf("expected calendar, got %q", kind)
	}
	if rolling != 0 {
		t.Fatalf("expected no rolling duration")
	}
	if calendar != calendarperiod.CalendarPeriod_WEEK {
		t.Fatalf("expected WEEK, got %v", calendar)
	}
}

func TestBuildPeriodInvalid(t *testing.T) {
	_, _, _, err := buildPeriod("calendar", "7d")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildBurnRateFilter(t *testing.T) {
	filter := buildBurnRateFilter("projects/p/services/s/serviceLevelObjectives/slo", "60m")
	want := "select_slo_burn_rate(\"projects/p/services/s/serviceLevelObjectives/slo\", \"60m\")"
	if filter != want {
		t.Fatalf("expected %q, got %q", want, filter)
	}
}
