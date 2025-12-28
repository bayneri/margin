package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bayneri/margin/internal/importer"
	"github.com/bayneri/margin/internal/monitoring"
	"gopkg.in/yaml.v3"
)

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "GCP project ID")
	service := fs.String("service", "", "Monitoring service ID")
	serviceType := fs.String("service-type", "", "margin service type (e.g., cloud-run)")
	outPath := fs.String("out", "", "output path for the imported spec")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*project) == "" {
		return errors.New("--project is required")
	}
	if strings.TrimSpace(*service) == "" {
		return errors.New("--service is required")
	}

	client, err := monitoring.NewGCPClient(context.Background())
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := importer.Import(context.Background(), client, importer.Options{
		Project:     *project,
		ServiceID:   *service,
		ServiceType: *serviceType,
	})
	if err != nil {
		return err
	}

	path := *outPath
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("out", "import", fmt.Sprintf("%s.yaml", *service))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(result.Spec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Wrote imported spec to %s\n", path)
	for _, warn := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warn)
	}
	if len(result.Warnings) > 0 {
		return exitError{code: 2, err: errors.New("partial import")}
	}
	return nil
}
