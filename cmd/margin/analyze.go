package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bayneri/margin/internal/analyze"
	"github.com/bayneri/margin/internal/report"
)

type analyzeOptions struct {
	project       string
	service       string
	start         string
	end           string
	last          string
	out           string
	format        string
	explain       bool
	timezone      string
	maxSLOs       int
	only          string
	failOnPartial bool
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := &analyzeOptions{}
	fs.StringVar(&opts.project, "project", "", "GCP project ID")
	fs.StringVar(&opts.service, "service", "", "Monitoring service ID or resource name")
	fs.StringVar(&opts.start, "start", "", "RFC3339 start time")
	fs.StringVar(&opts.end, "end", "", "RFC3339 end time")
	fs.StringVar(&opts.last, "last", "", "relative lookback duration (e.g. 90m, 6h)")
	fs.StringVar(&opts.out, "out", "", "output directory")
	fs.StringVar(&opts.format, "format", "md,json", "comma-separated output formats")
	fs.BoolVar(&opts.explain, "explain", false, "include formulas and query notes")
	fs.StringVar(&opts.timezone, "timezone", "UTC", "IANA timezone for reports")
	fs.IntVar(&opts.maxSLOs, "max-slos", 50, "maximum number of SLOs to analyze")
	fs.StringVar(&opts.only, "only", "", "regex to filter SLO display names or ids")
	fs.BoolVar(&opts.failOnPartial, "fail-on-partial", false, "exit non-zero if any SLO cannot be analyzed")

	if err := fs.Parse(args); err != nil {
		return err
	}

	lastDuration, err := analyze.ParseLast(opts.last)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(opts.timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	var only *regexp.Regexp
	if opts.only != "" {
		only, err = regexp.Compile(opts.only)
		if err != nil {
			return fmt.Errorf("invalid --only regex: %w", err)
		}
	}

	reader, err := analyze.NewGCPReader(context.Background())
	if err != nil {
		return err
	}
	defer reader.Close()

	result, sources, outDir, err := analyze.Run(context.Background(), reader, analyze.Options{
		Project:       opts.project,
		Service:       opts.service,
		Start:         opts.start,
		End:           opts.end,
		Last:          lastDuration,
		OutDir:        opts.out,
		Format:        parseFormat(opts.format),
		Explain:       opts.explain,
		Timezone:      loc,
		MaxSLOs:       opts.maxSLOs,
		Only:          only,
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	formats := parseFormat(opts.format)
	if includesFormat(formats, "md") {
		if err := report.WriteMarkdownSummary(filepath.Join(outDir, "summary.md"), result, report.Options{Explain: opts.explain, Timezone: loc}); err != nil {
			return err
		}
	}
	if includesFormat(formats, "json") {
		if err := report.WriteSummaryJSON(filepath.Join(outDir, "summary.json"), result); err != nil {
			return err
		}
	}
	if err := report.WriteSourcesJSON(filepath.Join(outDir, "sources.json"), sources); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		if err := report.WriteErrorsMarkdown(filepath.Join(outDir, "errors.md"), result.Errors); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stdout, "Wrote analysis to %s\n", outDir)

	if len(result.Errors) > 0 {
		if opts.failOnPartial {
			return exitError{code: 2, err: errors.New("partial analysis")}
		}
		fmt.Fprintln(os.Stdout, "Partial analysis: some SLOs could not be evaluated.")
		return nil
	}
	return nil
}

func parseFormat(input string) []string {
	if strings.TrimSpace(input) == "" {
		return []string{"md", "json"}
	}
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return []string{"md", "json"}
	}
	return out
}

func includesFormat(formats []string, value string) bool {
	for _, format := range formats {
		if format == value {
			return true
		}
	}
	return false
}
