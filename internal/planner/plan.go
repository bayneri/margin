package planner

import (
	"fmt"
	"sort"

	"github.com/bayneri/margin/internal/spec"
)

const ManagedByLabel = "managed-by"
const ManagedByValue = "margin"

type Plan struct {
	Project   string
	Service   string
	SLOs      []SLOPlan
	Alerts    []AlertPlan
	Dashboard DashboardPlan
}

type SLOPlan struct {
	ID        string
	Name      string
	Objective float64
	Window    string
	SLI       spec.SLI
	Labels    map[string]string
	Runbook   string
}

type AlertPlan struct {
	ID          string
	SLOName     string
	Type        string
	Windows     []string
	BurnRate    float64
	Severity    string
	Labels      map[string]string
	Runbook     string
	Description string
}

type DashboardPlan struct {
	ID      string
	Service string
	Labels  map[string]string
}

type Options struct {
	ProjectOverride string
	Labels          map[string]string
}

func Build(specDoc spec.Spec, opts Options) Plan {
	labels := mergeLabels(specDoc.Metadata.Labels, opts.Labels)
	labels[ManagedByLabel] = ManagedByValue

	project := specDoc.Metadata.Project
	if opts.ProjectOverride != "" {
		project = opts.ProjectOverride
	}

	var slos []SLOPlan
	for _, slo := range specDoc.SLOs {
		sloID := fmt.Sprintf("%s-%s", specDoc.Metadata.Name, slo.Name)
		slos = append(slos, SLOPlan{
			ID:        sloID,
			Name:      slo.Name,
			Objective: slo.Objective,
			Window:    slo.Window,
			SLI:       slo.SLI,
			Labels:    labels,
			Runbook:   specDoc.Metadata.Runbook,
		})
	}

	alerts := append(alertPlans(specDoc, labels), slowAlertPlans(specDoc, labels)...)

	return Plan{
		Project: project,
		Service: specDoc.Metadata.Service,
		SLOs:    slos,
		Alerts:  alerts,
		Dashboard: DashboardPlan{
			ID:      fmt.Sprintf("%s-dashboard", specDoc.Metadata.Name),
			Service: specDoc.Metadata.Name,
			Labels:  labels,
		},
	}
}

func mergeLabels(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func alertPlans(specDoc spec.Spec, labels map[string]string) []AlertPlan {
	return buildAlerts(specDoc, labels, "fast-burn", []string{"5m", "1h"}, 14.4, "page")
}

func slowAlertPlans(specDoc spec.Spec, labels map[string]string) []AlertPlan {
	return buildAlerts(specDoc, labels, "slow-burn", []string{"30m", "6h"}, 6.0, "ticket")
}

func buildAlerts(specDoc spec.Spec, labels map[string]string, alertType string, windows []string, burnRate float64, severity string) []AlertPlan {
	var alerts []AlertPlan
	for _, slo := range specDoc.SLOs {
		alertID := fmt.Sprintf("%s-%s-%s", specDoc.Metadata.Name, slo.Name, alertType)
		alerts = append(alerts, AlertPlan{
			ID:          alertID,
			SLOName:     slo.Name,
			Type:        alertType,
			Windows:     windows,
			BurnRate:    burnRate,
			Severity:    severity,
			Labels:      labels,
			Runbook:     specDoc.Metadata.Runbook,
			Description: fmt.Sprintf("%s burn alert for %s", alertType, slo.Name),
		})
	}
	return alerts
}

func SortedLabels(labels map[string]string) []string {
	var out []string
	for k, v := range labels {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(out)
	return out
}
