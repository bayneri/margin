package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bayneri/margin/internal/spec"
)

const ManagedByLabel = "managed-by"
const ManagedByValue = "margin"

type Plan struct {
	Project              string
	Service              string
	ServiceID            string
	ServiceName          string
	BurnRateResourceType string
	SLOs                 []SLOPlan
	Alerts               []AlertPlan
	Dashboard            DashboardPlan
}

type SLOPlan struct {
	ID          string
	ResourceID  string
	DisplayName string
	Name        string
	Objective   float64
	Window      string
	Period      string
	SLI         spec.SLI
	Labels      map[string]string
	Runbook     string
}

type AlertPlan struct {
	ID                   string
	DisplayName          string
	SLOName              string
	Type                 string
	Windows              []string
	BurnRate             float64
	Severity             string
	Labels               map[string]string
	Runbook              string
	Description          string
	BurnRateResourceType string
}

type DashboardPlan struct {
	ID          string
	DisplayName string
	Service     string
	Labels      map[string]string
}

type Options struct {
	ProjectOverride string
	Labels          map[string]string
}

func Build(specDoc spec.Spec, opts Options) Plan {
	labels := mergeLabels(specDoc.Metadata.Labels, opts.Labels)
	labels[ManagedByLabel] = ManagedByValue
	labels["service-name"] = specDoc.Metadata.Name

	burnRateResourceType := strings.TrimSpace(specDoc.Alerting.BurnRateResourceType)
	if burnRateResourceType == "" {
		burnRateResourceType = "global"
	}

	project := specDoc.Metadata.Project
	if opts.ProjectOverride != "" {
		project = opts.ProjectOverride
	}

	serviceID := sanitizeID(specDoc.Metadata.Name)

	var slos []SLOPlan
	for _, slo := range specDoc.SLOs {
		sloID := fmt.Sprintf("%s-%s", specDoc.Metadata.Name, slo.Name)
		sloResourceID := sanitizeID(sloID)
		displayName := sloID
		slos = append(slos, SLOPlan{
			ID:          sloID,
			ResourceID:  sloResourceID,
			DisplayName: displayName,
			Name:        slo.Name,
			Objective:   slo.Objective,
			Window:      slo.Window,
			Period:      slo.Period,
			SLI:         slo.SLI,
			Labels:      labels,
			Runbook:     specDoc.Metadata.Runbook,
		})
	}

	alerts := append(alertPlans(specDoc, labels, burnRateResourceType), slowAlertPlans(specDoc, labels, burnRateResourceType)...)

	return Plan{
		Project:              project,
		Service:              specDoc.Metadata.Service,
		ServiceID:            serviceID,
		ServiceName:          specDoc.Metadata.Name,
		BurnRateResourceType: burnRateResourceType,
		SLOs:                 slos,
		Alerts:               alerts,
		Dashboard: DashboardPlan{
			ID:          fmt.Sprintf("%s-dashboard", specDoc.Metadata.Name),
			DisplayName: fmt.Sprintf("%s service dashboard", specDoc.Metadata.Name),
			Service:     specDoc.Metadata.Name,
			Labels:      labels,
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

func alertPlans(specDoc spec.Spec, labels map[string]string, burnRateResourceType string) []AlertPlan {
	return buildAlerts(specDoc, labels, burnRateResourceType, "fast-burn", []string{"5m", "1h"}, 14.4, "page")
}

func slowAlertPlans(specDoc spec.Spec, labels map[string]string, burnRateResourceType string) []AlertPlan {
	return buildAlerts(specDoc, labels, burnRateResourceType, "slow-burn", []string{"30m", "6h"}, 6.0, "ticket")
}

func buildAlerts(specDoc spec.Spec, labels map[string]string, burnRateResourceType string, alertType string, windows []string, burnRate float64, severity string) []AlertPlan {
	var alerts []AlertPlan
	for _, slo := range specDoc.SLOs {
		alertID := fmt.Sprintf("%s-%s-%s", specDoc.Metadata.Name, slo.Name, alertType)
		displayName := fmt.Sprintf("%s %s %s", specDoc.Metadata.Name, slo.Name, alertType)
		alerts = append(alerts, AlertPlan{
			ID:                   alertID,
			DisplayName:          displayName,
			SLOName:              slo.Name,
			Type:                 alertType,
			Windows:              windows,
			BurnRate:             burnRate,
			Severity:             severity,
			Labels:               labels,
			Runbook:              specDoc.Metadata.Runbook,
			Description:          fmt.Sprintf("%s burn alert for %s", alertType, slo.Name),
			BurnRateResourceType: burnRateResourceType,
		})
	}
	return alerts
}

func sanitizeID(input string) string {
	normalized := strings.ToLower(input)
	var out []rune
	lastDash := false
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	result := strings.Trim(string(out), "-")
	if result == "" {
		return "service"
	}
	return result
}

func SortedLabels(labels map[string]string) []string {
	var out []string
	for k, v := range labels {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(out)
	return out
}
