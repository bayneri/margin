package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bayneri/margin/internal/analyze"
)

type Options struct {
	Explain  bool
	Timezone *time.Location
}

func WriteMarkdownSummary(path string, result analyze.Result, opts Options) error {
	if opts.Timezone == nil {
		opts.Timezone = time.UTC
	}
	var b strings.Builder
	windowStart := result.Window.Start.In(opts.Timezone).Format(time.RFC3339)
	windowEnd := result.Window.End.In(opts.Timezone).Format(time.RFC3339)

	fmt.Fprintf(&b, "# Incident window analysis\n\n")
	fmt.Fprintf(&b, "- Service: %s\n", result.Service)
	fmt.Fprintf(&b, "- Project: %s\n", result.Project)
	fmt.Fprintf(&b, "- Window: %s to %s\n", windowStart, windowEnd)
	fmt.Fprintf(&b, "- Duration: %s\n\n", formatDuration(result.Window.DurationSeconds))

	fmt.Fprintf(&b, "| SLO | Goal | Compliance | Bad fraction | Allowed bad | Budget consumed | Status |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, slo := range result.SLOs {
		fmt.Fprintf(&b, "| %s | %.4f | %.4f | %.4f | %.4f | %.2f%% | %s |\n",
			slo.DisplayName, slo.Goal, slo.Compliance, slo.BadFraction, slo.AllowedBadFraction, slo.ConsumedPercentOfBudget, slo.Status)
	}

	if len(result.Errors) > 0 {
		fmt.Fprintf(&b, "\n## Notes & assumptions\n")
		for _, err := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", err)
		}
	}

	if opts.Explain {
		fmt.Fprintf(&b, "\n## How computed\n")
		fmt.Fprintf(&b, "\nFormula: allowedBad = 1 - goal; bad = 1 - compliance; consumedPercent = (bad / allowedBad) * 100\n")
		for _, slo := range result.SLOs {
			if slo.Explain == nil || len(slo.Explain.Notes) == 0 {
				continue
			}
			fmt.Fprintf(&b, "\n### %s\n", slo.DisplayName)
			for _, note := range slo.Explain.Notes {
				fmt.Fprintf(&b, "- %s\n", note)
			}
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func formatDuration(seconds int64) string {
	return (time.Duration(seconds) * time.Second).String()
}
