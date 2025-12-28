package report

import (
	"os"
	"testing"
	"time"

	"github.com/bayneri/margin/internal/analyze"
)

func TestWriteMarkdownSummary(t *testing.T) {
	result := analyze.Result{
		Project: "demo",
		Service: "checkout",
		Window: analyze.Window{
			Start:           time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			End:             time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			DurationSeconds: 3600,
		},
		SLOs: []analyze.SLOResult{{
			DisplayName:             "availability",
			Goal:                    0.999,
			Compliance:              0.995,
			BadFraction:             0.005,
			AllowedBadFraction:      0.001,
			ConsumedPercentOfBudget: 500,
			Status:                  "ok",
		}},
	}

	path := "./test-summary.md"
	defer os.Remove(path)

	if err := WriteMarkdownSummary(path, result, Options{Explain: true, Timezone: time.UTC}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	golden, err := os.ReadFile("./testdata/summary.md")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(data) != string(golden) {
		t.Fatalf("markdown mismatch")
	}
}
