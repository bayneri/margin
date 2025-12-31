package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/bayneri/margin/internal/export/monitoringjson"
	"github.com/bayneri/margin/internal/export/terraform"
	"github.com/bayneri/margin/internal/spec"
)

func runExport(args []string) error {
	if len(args) == 0 {
		return errors.New("export requires a format: terraform")
	}
	switch args[0] {
	case "terraform":
		return runExportTerraform(args[1:])
	case "monitoring-json":
		return runExportMonitoringJSON(args[1:])
	default:
		return fmt.Errorf("unknown export format %q", args[0])
	}
}

func runExportTerraform(args []string) error {
	fs, opts := baseFlags("export terraform", args)
	outDir := fs.String("out", "out/terraform", "output directory")
	module := fs.Bool("module", false, "emit Terraform module (main/variables/outputs)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, specDoc, err := buildPlan(opts)
	if err != nil {
		return err
	}
	template, err := spec.TemplateForService(specDoc.Metadata.Service)
	if err != nil {
		return err
	}
	if *module && *outDir == "out/terraform" {
		*outDir = "out/terraform-module"
	}
	if *module {
		path, err := terraform.WriteModule(plan, template, *outDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Wrote Terraform module to %s\n", path)
		return nil
	}
	path, err := terraform.Write(plan, template, *outDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Wrote Terraform export to %s\n", path)
	return nil
}

func runExportMonitoringJSON(args []string) error {
	fs, opts := baseFlags("export monitoring-json", args)
	outDir := fs.String("out", "out/monitoring-json", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, specDoc, err := buildPlan(opts)
	if err != nil {
		return err
	}
	template, err := spec.TemplateForService(specDoc.Metadata.Service)
	if err != nil {
		return err
	}
	path, err := monitoringjson.Write(plan, template, *outDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Wrote Monitoring JSON export to %s\n", path)
	return nil
}
