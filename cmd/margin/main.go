package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bayneri/margin/internal/alerting"
	"github.com/bayneri/margin/internal/monitoring"
	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

const version = "0.1.0"

type commandOptions struct {
	file    string
	project string
	dryRun  bool
	verbose bool
	labels  string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "analyze":
		if err := runAnalyze(os.Args[2:]); err != nil {
			fail(err)
		}
	case "apply":
		if err := runApply(os.Args[2:]); err != nil {
			fail(err)
		}
	case "plan":
		if err := runPlan(os.Args[2:]); err != nil {
			fail(err)
		}
	case "validate":
		if err := runValidate(os.Args[2:]); err != nil {
			fail(err)
		}
	case "explain":
		if err := runExplain(os.Args[2:]); err != nil {
			fail(err)
		}
	case "delete":
		if err := runDelete(os.Args[2:]); err != nil {
			fail(err)
		}
	case "version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "margin - opinionated SLOs for Google Cloud")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  margin apply   -f slo.yaml")
	fmt.Fprintln(os.Stderr, "  margin analyze --project my-gcp-project --service checkout-api --last 90m")
	fmt.Fprintln(os.Stderr, "  margin plan    -f slo.yaml")
	fmt.Fprintln(os.Stderr, "  margin validate -f slo.yaml")
	fmt.Fprintln(os.Stderr, "  margin explain burn-rate")
	fmt.Fprintln(os.Stderr, "  margin delete  -f slo.yaml")
}

func baseFlags(cmd string, args []string) (*flag.FlagSet, *commandOptions) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := &commandOptions{}
	fs.StringVar(&opts.file, "f", "", "path to SLO spec")
	fs.StringVar(&opts.project, "project", "", "GCP project ID (overrides metadata.project)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show planned changes without applying")
	fs.BoolVar(&opts.verbose, "verbose", false, "verbose output")
	fs.StringVar(&opts.labels, "labels", "", "extra labels in key=value,key=value format")
	return fs, opts
}

func runApply(args []string) error {
	fs, opts := baseFlags("apply", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, specDoc, err := buildPlan(opts)
	if err != nil {
		return err
	}
	if opts.dryRun {
		planner.Render(os.Stdout, plan)
		return nil
	}
	client, err := monitoring.NewGCPClient(context.Background())
	if err != nil {
		return err
	}
	defer client.Close()

	if err := monitoring.ApplyPlan(context.Background(), client, plan); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Applied %d SLOs, %d alerts, and 1 dashboard in project %s.\n", len(plan.SLOs), len(plan.Alerts), plan.Project)
	fmt.Fprintf(os.Stdout, "Cloud Console: https://console.cloud.google.com/monitoring/services?project=%s\n", plan.Project)
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "Loaded spec for %s with %d SLOs.\n", specDoc.Metadata.Name, len(specDoc.SLOs))
	}
	return nil
}

func runPlan(args []string) error {
	fs, opts := baseFlags("plan", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, _, err := buildPlan(opts)
	if err != nil {
		return err
	}
	planner.Render(os.Stdout, plan)
	return nil
}

func runValidate(args []string) error {
	fs, opts := baseFlags("validate", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, _, err := buildPlan(opts)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Spec is valid.")
	return nil
}

func runExplain(args []string) error {
	if len(args) != 1 {
		return errors.New("explain requires a topic: burn-rate")
	}
	if args[0] != "burn-rate" {
		return fmt.Errorf("unknown explain topic %q", args[0])
	}
	fmt.Fprintln(os.Stdout, alerting.ExplainBurnRate())
	return nil
}

func runDelete(args []string) error {
	fs, opts := baseFlags("delete", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, _, err := buildPlan(opts)
	if err != nil {
		return err
	}
	if opts.dryRun {
		fmt.Fprintf(os.Stdout, "Delete would remove %d SLOs, %d alerts, and 1 dashboard in project %s.\n", len(plan.SLOs), len(plan.Alerts), plan.Project)
		return nil
	}
	client, err := monitoring.NewGCPClient(context.Background())
	if err != nil {
		return err
	}
	defer client.Close()
	if err := monitoring.DeletePlan(context.Background(), client, plan); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Deleted managed resources for %s in project %s.\n", plan.ServiceName, plan.Project)
	return nil
}

func buildPlan(opts *commandOptions) (planner.Plan, spec.Spec, error) {
	if strings.TrimSpace(opts.file) == "" {
		return planner.Plan{}, spec.Spec{}, errors.New("-f is required")
	}
	labels, err := spec.ParseLabels(opts.labels)
	if err != nil {
		return planner.Plan{}, spec.Spec{}, err
	}
	specDoc, err := spec.Load(opts.file)
	if err != nil {
		return planner.Plan{}, spec.Spec{}, err
	}
	if err := specDoc.Validate(); err != nil {
		return planner.Plan{}, spec.Spec{}, err
	}
	if strings.TrimSpace(opts.project) == "" && strings.TrimSpace(specDoc.Metadata.Project) == "" {
		return planner.Plan{}, spec.Spec{}, errors.New("project is required via --project or metadata.project")
	}
	if opts.project != "" && specDoc.Metadata.Project != "" && opts.project != specDoc.Metadata.Project {
		return planner.Plan{}, spec.Spec{}, fmt.Errorf("--project %q does not match metadata.project %q", opts.project, specDoc.Metadata.Project)
	}

	plan := planner.Build(specDoc, planner.Options{
		ProjectOverride: opts.project,
		Labels:          labels,
	})
	return plan, specDoc, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	if err == nil {
		os.Exit(1)
	}
	type exitCoder interface {
		ExitCode() int
	}
	if coded, ok := err.(exitCoder); ok {
		os.Exit(coded.ExitCode())
	}
	os.Exit(1)
}
