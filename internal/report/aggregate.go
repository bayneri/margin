package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bayneri/margin/internal/analyze"
)

type AggregateOptions struct {
	Timezone *time.Location
}

type AggregateResult struct {
	SchemaVersion string             `json:"schemaVersion"`
	Inputs        []string           `json:"inputs"`
	Status        string             `json:"status"`
	Services      []ServiceAggregate `json:"services"`
	Errors        []string           `json:"errors"`
}

type ServiceAggregate struct {
	Project string              `json:"project"`
	Service string              `json:"service"`
	Status  string              `json:"status"`
	Window  analyze.Window      `json:"window"`
	SLOs    []analyze.SLOResult `json:"slos"`
	Errors  []string            `json:"errors"`
}

func ReadResults(paths []string) ([]analyze.Result, error) {
	var results []analyze.Result
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var result analyze.Result
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if result.SchemaVersion == "" {
			return nil, fmt.Errorf("missing schemaVersion in %s", path)
		}
		results = append(results, result)
	}
	return results, nil
}

func Aggregate(results []analyze.Result, inputs []string) (AggregateResult, error) {
	if len(results) == 0 {
		return AggregateResult{}, errors.New("no results to aggregate")
	}
	byService := map[string]*ServiceAggregate{}
	var errorsList []string
	status := analyze.StatusOK
	for i, result := range results {
		status = mergeStatus(status, result.Status)
		status = mergeStatus(status, statusFromSLOs(result.SLOs))
		key := fmt.Sprintf("%s/%s", result.Project, result.Service)
		item, ok := byService[key]
		if !ok {
			item = &ServiceAggregate{
				Project: result.Project,
				Service: result.Service,
				Status:  result.Status,
				Window:  result.Window,
			}
			byService[key] = item
		} else {
			if item.Window.Start != result.Window.Start || item.Window.End != result.Window.End {
				errorsList = append(errorsList, fmt.Sprintf("window mismatch for %s between inputs", key))
			}
			item.Status = mergeStatus(item.Status, result.Status)
		}
		item.SLOs = append(item.SLOs, result.SLOs...)
		item.Errors = append(item.Errors, result.Errors...)
		item.Status = mergeStatus(item.Status, statusFromSLOs(result.SLOs))
		if len(result.Errors) > 0 && len(inputs) > i {
			errorsList = append(errorsList, fmt.Sprintf("%s: %d error(s)", inputs[i], len(result.Errors)))
		}
	}

	var services []ServiceAggregate
	for _, item := range byService {
		sort.Slice(item.SLOs, func(i, j int) bool {
			if item.SLOs[i].DisplayName == item.SLOs[j].DisplayName {
				return item.SLOs[i].SLOResourceName < item.SLOs[j].SLOResourceName
			}
			return item.SLOs[i].DisplayName < item.SLOs[j].DisplayName
		})
		services = append(services, *item)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].Project == services[j].Project {
			return services[i].Service < services[j].Service
		}
		return services[i].Project < services[j].Project
	})

	return AggregateResult{
		SchemaVersion: analyze.SchemaVersion,
		Inputs:        inputs,
		Status:        status,
		Services:      services,
		Errors:        errorsList,
	}, nil
}

func mergeStatus(a, b string) string {
	score := func(value string) int {
		switch value {
		case analyze.StatusBreach:
			return 3
		case analyze.StatusPartial, analyze.StatusError:
			return 2
		case analyze.StatusOK:
			return 1
		default:
			return 0
		}
	}
	if score(b) > score(a) {
		return b
	}
	return a
}

func statusFromSLOs(slos []analyze.SLOResult) string {
	status := analyze.StatusOK
	for _, slo := range slos {
		status = mergeStatus(status, slo.Status)
	}
	return status
}

func WriteAggregateJSON(path string, result AggregateResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func WriteAggregateMarkdown(path string, result AggregateResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# margin report\n\n")
	fmt.Fprintf(&b, "Inputs: %d\n\n", len(result.Inputs))
	for _, service := range result.Services {
		fmt.Fprintf(&b, "## %s (%s)\n\n", service.Service, service.Project)
		fmt.Fprintf(&b, "- Status: %s\n", service.Status)
		fmt.Fprintf(&b, "- Window: %s to %s\n\n", service.Window.Start.Format(time.RFC3339), service.Window.End.Format(time.RFC3339))

		fmt.Fprintf(&b, "| SLO | Goal | Compliance | Bad fraction | Allowed bad | Budget consumed | Status |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, slo := range service.SLOs {
			fmt.Fprintf(&b, "| %s | %.4f | %.4f | %.4f | %.4f | %.2f%% | %s |\n",
				slo.DisplayName, slo.Goal, slo.Compliance, slo.BadFraction, slo.AllowedBadFraction, slo.ConsumedPercentOfBudget, slo.Status)
		}
		if len(service.Errors) > 0 {
			fmt.Fprintf(&b, "\nErrors:\n")
			for _, err := range service.Errors {
				fmt.Fprintf(&b, "- %s\n", err)
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(&b, "## Warnings\n")
		for _, err := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", err)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
