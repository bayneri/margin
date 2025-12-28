package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bayneri/margin/internal/analyze"
)

func TestAggregateResults(t *testing.T) {
	window := analyze.Window{
		Start:           time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		End:             time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
		DurationSeconds: 3600,
	}
	results := []analyze.Result{
		{
			SchemaVersion: analyze.SchemaVersion,
			Project:       "demo",
			Service:       "checkout",
			Status:        analyze.StatusOK,
			Window:        window,
			SLOs: []analyze.SLOResult{{
				DisplayName:             "availability",
				Goal:                    0.999,
				Compliance:              0.995,
				BadFraction:             0.005,
				AllowedBadFraction:      0.001,
				ConsumedPercentOfBudget: 500,
				Status:                  analyze.StatusBreach,
			}},
		},
		{
			SchemaVersion: analyze.SchemaVersion,
			Project:       "demo",
			Service:       "checkout",
			Status:        analyze.StatusPartial,
			Window:        window,
			SLOs: []analyze.SLOResult{{
				DisplayName:             "latency",
				Goal:                    0.99,
				Compliance:              0.98,
				BadFraction:             0.02,
				AllowedBadFraction:      0.01,
				ConsumedPercentOfBudget: 200,
				Status:                  analyze.StatusPartial,
			}},
			Errors: []string{"latency query failed"},
		},
	}
	agg, err := Aggregate(results, []string{"one.json", "two.json"})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(agg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(agg.Services))
	}
	if agg.Status != analyze.StatusBreach {
		t.Fatalf("expected overall breach, got %s", agg.Status)
	}
	if agg.Services[0].Status != analyze.StatusBreach {
		t.Fatalf("expected breach, got %s", agg.Services[0].Status)
	}

	dir := t.TempDir()
	md := filepath.Join(dir, "summary.md")
	if err := WriteAggregateMarkdown(md, agg); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	if _, err := os.Stat(md); err != nil {
		t.Fatalf("markdown missing: %v", err)
	}

	js := filepath.Join(dir, "summary.json")
	if err := WriteAggregateJSON(js, agg); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if _, err := os.Stat(js); err != nil {
		t.Fatalf("json missing: %v", err)
	}
}
