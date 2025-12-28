package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bayneri/margin/internal/monitoring"
)

func runServices(args []string) error {
	if len(args) == 0 {
		return errors.New("services requires a subcommand: list")
	}
	switch args[0] {
	case "list":
		return runServicesList(args[1:])
	default:
		return fmt.Errorf("unknown services subcommand %q", args[0])
	}
}

func runServicesList(args []string) error {
	fs := flag.NewFlagSet("services list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "GCP project ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*project) == "" {
		return errors.New("--project is required")
	}

	client, err := monitoring.NewGCPClient(context.Background())
	if err != nil {
		return err
	}
	defer client.Close()

	services, err := client.ListServices(context.Background(), *project)
	if err != nil {
		return err
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].DisplayName == services[j].DisplayName {
			return services[i].Name < services[j].Name
		}
		return services[i].DisplayName < services[j].DisplayName
	})

	fmt.Fprintln(os.Stdout, "SERVICE_ID\tDISPLAY_NAME\tRESOURCE_NAME")
	for _, svc := range services {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", serviceIDFromName(svc.Name), svc.DisplayName, svc.Name)
	}
	return nil
}

func serviceIDFromName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "/services/")
	if len(parts) != 2 {
		return name
	}
	return parts[1]
}
