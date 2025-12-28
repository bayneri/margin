package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bayneri/margin/internal/report"
)

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	inputs := fs.String("inputs", "", "comma-separated list of analyze summary.json files")
	outDir := fs.String("out", "out/report", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*inputs) == "" {
		return errors.New("--inputs is required")
	}

	paths := splitCSV(*inputs)
	results, err := report.ReadResults(paths)
	if err != nil {
		return err
	}
	agg, err := report.Aggregate(results, paths)
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(*outDir, "summary.json")
	if err := report.WriteAggregateJSON(jsonPath, agg); err != nil {
		return err
	}
	mdPath := filepath.Join(*outDir, "summary.md")
	if err := report.WriteAggregateMarkdown(mdPath, agg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Wrote report to %s\n", *outDir)
	if len(agg.Errors) > 0 {
		return exitError{code: 2, err: errors.New("partial report")}
	}
	return nil
}

func splitCSV(input string) []string {
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
