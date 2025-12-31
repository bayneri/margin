package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bayneri/margin/internal/monitoring"
	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

const outputFile = "main.tf.json"
const moduleMainFile = "main.tf.json"
const moduleVariablesFile = "variables.tf.json"
const moduleOutputsFile = "outputs.tf.json"

func Write(plan planner.Plan, template spec.ServiceTemplate, outDir string) (string, error) {
	if outDir == "" {
		outDir = filepath.Join("out", "terraform")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	dashboardJSON, err := monitoring.BuildDashboardJSON(monitoring.ApplyDashboardRequest{
		Project:   plan.Project,
		ServiceID: plan.ServiceID,
		Dashboard: plan.Dashboard,
		SLOs:      plan.SLOs,
		Template:  template,
		Labels:    plan.Dashboard.Labels,
	})
	if err != nil {
		return "", err
	}

	cfg := map[string]interface{}{
		"terraform": map[string]interface{}{
			"required_providers": map[string]interface{}{
				"google": map[string]interface{}{
					"source":  "hashicorp/google",
					"version": ">= 5.0",
				},
			},
		},
		"provider": map[string]interface{}{
			"google": map[string]interface{}{
				"project": plan.Project,
			},
		},
		"resource": buildResourcesForProject(plan, template, dashboardJSON, plan.Project),
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	path := filepath.Join(outDir, outputFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// WriteModule emits a Terraform module skeleton (main + variables + outputs),
// parameterizing the project as a variable.
func WriteModule(plan planner.Plan, template spec.ServiceTemplate, outDir string) (string, error) {
	if outDir == "" {
		outDir = filepath.Join("out", "terraform-module")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	dashboardJSON, err := monitoring.BuildDashboardJSON(monitoring.ApplyDashboardRequest{
		Project:   plan.Project,
		ServiceID: plan.ServiceID,
		Dashboard: plan.Dashboard,
		SLOs:      plan.SLOs,
		Template:  template,
		Labels:    plan.Dashboard.Labels,
	})
	if err != nil {
		return "", err
	}

	projectExpr := "${var.project}"
	resources := buildResourcesForProject(plan, template, dashboardJSON, projectExpr)

	mainCfg := map[string]interface{}{
		"terraform": map[string]interface{}{
			"required_providers": map[string]interface{}{
				"google": map[string]interface{}{
					"source":  "hashicorp/google",
					"version": ">= 5.0",
				},
			},
		},
		"resource": resources,
	}
	mainData, err := json.MarshalIndent(mainCfg, "", "  ")
	if err != nil {
		return "", err
	}
	mainData = append(mainData, '\n')
	if err := os.WriteFile(filepath.Join(outDir, moduleMainFile), mainData, 0644); err != nil {
		return "", err
	}

	vars := map[string]interface{}{
		"variable": map[string]interface{}{
			"project": map[string]interface{}{
				"type": "string",
			},
		},
	}
	varData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return "", err
	}
	varData = append(varData, '\n')
	if err := os.WriteFile(filepath.Join(outDir, moduleVariablesFile), varData, 0644); err != nil {
		return "", err
	}

	serviceName := tfName("service", plan.ServiceID)
	var sloNames, alertNames []string
	for _, slo := range plan.SLOs {
		sloNames = append(sloNames, tfName("slo", slo.ResourceID))
	}
	for _, alert := range plan.Alerts {
		alertNames = append(alertNames, tfName("alert", alert.ID))
	}
	dashName := tfName("dashboard", plan.Dashboard.ID)

	outputs := map[string]interface{}{
		"output": map[string]interface{}{
			"service_name": map[string]interface{}{
				"value": fmt.Sprintf("${google_monitoring_service.%s.name}", serviceName),
			},
			"slo_names": map[string]interface{}{
				"value": buildOutputRefs("google_monitoring_slo", sloNames, "name"),
			},
			"alert_policy_names": map[string]interface{}{
				"value": buildOutputRefs("google_monitoring_alert_policy", alertNames, "name"),
			},
			"dashboard_name": map[string]interface{}{
				"value": fmt.Sprintf("${google_monitoring_dashboard.%s.name}", dashName),
			},
		},
	}
	outData, err := json.MarshalIndent(outputs, "", "  ")
	if err != nil {
		return "", err
	}
	outData = append(outData, '\n')
	if err := os.WriteFile(filepath.Join(outDir, moduleOutputsFile), outData, 0644); err != nil {
		return "", err
	}

	return filepath.Join(outDir, moduleMainFile), nil
}

func buildOutputRefs(resourceType string, names []string, attr string) interface{} {
	if len(names) == 0 {
		return []string{}
	}
	var refs []string
	for _, name := range names {
		refs = append(refs, fmt.Sprintf("${%s.%s.%s}", resourceType, name, attr))
	}
	return refs
}

func buildResources(plan planner.Plan, template spec.ServiceTemplate, dashboardJSON string) map[string]map[string]interface{} {
	return buildResourcesForProject(plan, template, dashboardJSON, plan.Project)
}

func buildResourcesForProject(plan planner.Plan, template spec.ServiceTemplate, dashboardJSON string, projectValue string) map[string]map[string]interface{} {
	resources := map[string]map[string]interface{}{}

	serviceName := tfName("service", plan.ServiceID)
	resources["google_monitoring_service"] = map[string]interface{}{
		serviceName: map[string]interface{}{
			"project":      projectValue,
			"service_id":   plan.ServiceID,
			"display_name": plan.ServiceName,
			"user_labels":  plan.Dashboard.Labels,
		},
	}

	sloResources := map[string]interface{}{}
	for _, slo := range plan.SLOs {
		sloResources[tfName("slo", slo.ResourceID)] = buildSLOResource(plan, slo, template)
	}
	if len(sloResources) > 0 {
		resources["google_monitoring_slo"] = sloResources
	}

	alertResources := map[string]interface{}{}
	for _, alert := range plan.Alerts {
		alertResources[tfName("alert", alert.ID)] = buildAlertResource(plan, alert)
	}
	if len(alertResources) > 0 {
		resources["google_monitoring_alert_policy"] = alertResources
	}

	resources["google_monitoring_dashboard"] = map[string]interface{}{
		tfName("dashboard", plan.Dashboard.ID): map[string]interface{}{
			"dashboard_json": dashboardJSON,
		},
	}

	return resources
}

func buildSLOResource(plan planner.Plan, slo planner.SLOPlan, template spec.ServiceTemplate) map[string]interface{} {
	return buildSLOResourceWithProject(plan, slo, template, plan.Project)
}

func buildSLOResourceWithProject(plan planner.Plan, slo planner.SLOPlan, template spec.ServiceTemplate, projectValue string) map[string]interface{} {
	resource := map[string]interface{}{
		"project":      projectValue,
		"service":      fmt.Sprintf("projects/%s/services/%s", projectValue, plan.ServiceID),
		"slo_id":       slo.ResourceID,
		"display_name": slo.DisplayName,
		"goal":         slo.Objective / 100.0,
		"user_labels":  slo.Labels,
	}

	period := strings.TrimSpace(slo.Period)
	if period == "" || period == "rolling" {
		days, err := rollingDays(slo.Window)
		if err == nil {
			resource["rolling_period_days"] = days
		}
	} else if period == "calendar" {
		if cal := calendarPeriod(slo.Window); cal != "" {
			resource["calendar_period"] = cal
		}
	}

	switch slo.SLI.Type {
	case "request-based":
		resource["request_based_sli"] = map[string]interface{}{
			"good_total_ratio": map[string]interface{}{
				"good_service_filter":  buildFilter(slo.SLI.Good.Metric, template.ResourceType, slo.SLI.Good.Filter),
				"total_service_filter": buildFilter(slo.SLI.Total.Metric, template.ResourceType, slo.SLI.Total.Filter),
			},
		}
	case "latency":
		threshold, err := parseThreshold(slo.SLI.Threshold)
		if err == nil {
			resource["request_based_sli"] = map[string]interface{}{
				"distribution_cut": map[string]interface{}{
					"distribution_filter": buildFilter(slo.SLI.Metric, template.ResourceType, slo.SLI.Filter),
					"range": map[string]interface{}{
						"min": 0,
						"max": threshold,
					},
				},
			}
		}
	}

	return resource
}

func buildAlertResource(plan planner.Plan, alert planner.AlertPlan) map[string]interface{} {
	return buildAlertResourceWithProject(plan, alert, plan.Project)
}

func buildAlertResourceWithProject(plan planner.Plan, alert planner.AlertPlan, projectValue string) map[string]interface{} {
	conditions := []map[string]interface{}{}
	for _, window := range alert.Windows {
		duration, err := parseWindow(window)
		if err != nil {
			continue
		}
		conditions = append(conditions, map[string]interface{}{
			"display_name": fmt.Sprintf("%s %s", alert.DisplayName, window),
			"condition_threshold": map[string]interface{}{
				"filter":                  buildBurnRateFilter(sloRef(plan, alert.SLOName), window),
				"comparison":              "COMPARISON_GT",
				"threshold_value":         alert.BurnRate,
				"duration":                formatDuration(duration),
				"evaluation_missing_data": "EVALUATION_MISSING_DATA_NO_OP",
			},
		})
	}

	return map[string]interface{}{
		"project":      projectValue,
		"display_name": alert.DisplayName,
		"combiner":     "AND",
		"documentation": map[string]interface{}{
			"content":   buildAlertDocumentation(alert, alert.SLOName),
			"mime_type": "text/markdown",
		},
		"conditions":  conditions,
		"user_labels": alert.Labels,
		"enabled":     true,
		"severity":    severity(alert.Severity),
	}
}

func buildAlertDocumentation(alert planner.AlertPlan, sloName string) string {
	lines := []string{
		fmt.Sprintf("SLO: %s", sloName),
		fmt.Sprintf("Alert type: %s", alert.Type),
		fmt.Sprintf("Burn rate: %.1fx", alert.BurnRate),
		fmt.Sprintf("Windows: %s", strings.Join(alert.Windows, ", ")),
		fmt.Sprintf("Runbook: %s", alert.Runbook),
	}
	return strings.Join(lines, "\n")
}

func severity(value string) string {
	switch strings.ToLower(value) {
	case "page":
		return "CRITICAL"
	case "ticket":
		return "WARNING"
	default:
		return "SEVERITY_UNSPECIFIED"
	}
}

func sloRef(plan planner.Plan, sloName string) string {
	for _, slo := range plan.SLOs {
		if slo.Name == sloName {
			return fmt.Sprintf("projects/%s/services/%s/serviceLevelObjectives/%s", plan.Project, plan.ServiceID, slo.ResourceID)
		}
	}
	return ""
}

func buildFilter(metric, resourceType, extra string) string {
	filter := fmt.Sprintf("metric.type=%q AND resource.type=%q", metric, resourceType)
	if strings.TrimSpace(extra) != "" {
		return fmt.Sprintf("%s AND %s", filter, extra)
	}
	return filter
}

func buildBurnRateFilter(sloRef, window string) string {
	return fmt.Sprintf("select_slo_burn_rate(%q, %q)", sloRef, window)
}

func rollingDays(window string) (int64, error) {
	duration, err := parseWindow(window)
	if err != nil {
		return 0, err
	}
	if duration%(24*time.Hour) != 0 {
		return 0, fmt.Errorf("window %q must be whole days for terraform export", window)
	}
	return int64(duration.Hours() / 24), nil
}

func calendarPeriod(window string) string {
	switch window {
	case "1d":
		return "DAY"
	case "1w":
		return "WEEK"
	case "2w":
		return "FORTNIGHT"
	case "30d":
		return "MONTH"
	default:
		return ""
	}
}

func parseWindow(window string) (time.Duration, error) {
	if window == "" {
		return 0, fmt.Errorf("window is empty")
	}
	unit := window[len(window)-1]
	value := window[:len(window)-1]
	var amount int
	if _, err := fmt.Sscanf(value, "%d", &amount); err != nil {
		return 0, fmt.Errorf("invalid window %q", window)
	}
	switch unit {
	case 's':
		return time.Duration(amount) * time.Second, nil
	case 'm':
		return time.Duration(amount) * time.Minute, nil
	case 'h':
		return time.Duration(amount) * time.Hour, nil
	case 'd':
		return time.Duration(amount) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(amount) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown window unit %q", string(unit))
	}
}

func parseThreshold(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("threshold is empty")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid threshold %q", value)
	}
	return parsed.Seconds(), nil
}

func formatDuration(duration time.Duration) string {
	seconds := int64(duration.Seconds())
	if seconds < 0 {
		seconds = -seconds
	}
	return fmt.Sprintf("%ds", seconds)
}

func tfName(prefix, value string) string {
	normalized := strings.ToLower(value)
	var out []rune
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 || (out[0] >= '0' && out[0] <= '9') {
		return fmt.Sprintf("%s_%s", prefix, string(out))
	}
	return string(out)
}
