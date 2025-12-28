package monitoringjson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bayneri/margin/internal/monitoring"
	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const outputFile = "monitoring.json"

func Write(plan planner.Plan, template spec.ServiceTemplate, outDir string) (string, error) {
	if outDir == "" {
		outDir = filepath.Join("out", "monitoring-json")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	service := monitoring.BuildService(monitoring.EnsureServiceRequest{
		Project:     plan.Project,
		ServiceID:   plan.ServiceID,
		DisplayName: plan.ServiceName,
		Labels:      plan.Dashboard.Labels,
	})
	serviceJSON, err := protoToInterface(service)
	if err != nil {
		return "", err
	}

	var slos []interface{}
	for _, slo := range plan.SLOs {
		obj, err := monitoring.BuildSLO(monitoring.ApplySLORequest{
			Project:   plan.Project,
			ServiceID: plan.ServiceID,
			SLO:       slo,
			Template:  template,
			Labels:    slo.Labels,
		})
		if err != nil {
			return "", err
		}
		item, err := protoToInterface(obj)
		if err != nil {
			return "", err
		}
		slos = append(slos, item)
	}

	sloRefs := map[string]string{}
	for _, slo := range plan.SLOs {
		sloRefs[slo.Name] = fmt.Sprintf("projects/%s/services/%s/serviceLevelObjectives/%s", plan.Project, plan.ServiceID, slo.ResourceID)
	}

	var alerts []interface{}
	for _, alert := range plan.Alerts {
		ref := sloRefs[alert.SLOName]
		obj, err := monitoring.BuildAlertPolicy(monitoring.ApplyAlertRequest{
			Project: plan.Project,
			SLOName: alert.SLOName,
			SLORef:  ref,
			Alert:   alert,
			Labels:  alert.Labels,
		})
		if err != nil {
			return "", err
		}
		item, err := protoToInterface(obj)
		if err != nil {
			return "", err
		}
		alerts = append(alerts, item)
	}

	dashboard := monitoring.BuildDashboard(monitoring.ApplyDashboardRequest{
		Project:   plan.Project,
		ServiceID: plan.ServiceID,
		Dashboard: plan.Dashboard,
		SLOs:      plan.SLOs,
		Template:  template,
		Labels:    plan.Dashboard.Labels,
	})
	dashboardJSON, err := protoToInterface(dashboard)
	if err != nil {
		return "", err
	}

	payload := map[string]interface{}{
		"service":       serviceJSON,
		"slos":          slos,
		"alertPolicies": alerts,
		"dashboard":     dashboardJSON,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
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

func protoToInterface(msg proto.Message) (interface{}, error) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
