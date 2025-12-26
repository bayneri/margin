package monitoring

import (
	"context"
	"fmt"

	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

func ApplyPlan(ctx context.Context, client Client, plan planner.Plan) error {
	template, err := spec.TemplateForService(plan.Service)
	if err != nil {
		return fmt.Errorf("load service template: %w", err)
	}

	if err := client.EnsureService(ctx, EnsureServiceRequest{
		Project:     plan.Project,
		ServiceID:   plan.ServiceID,
		DisplayName: plan.ServiceName,
		Labels:      plan.Dashboard.Labels,
	}); err != nil {
		return fmt.Errorf("ensure service: %w", err)
	}

	sloRefs := map[string]string{}
	for _, slo := range plan.SLOs {
		ref, err := client.ApplySLO(ctx, ApplySLORequest{
			Project:   plan.Project,
			ServiceID: plan.ServiceID,
			SLO:       slo,
			Template:  template,
			Labels:    slo.Labels,
		})
		if err != nil {
			return fmt.Errorf("apply SLO %s: %w", slo.Name, err)
		}
		sloRefs[slo.Name] = ref
	}

	for _, alert := range plan.Alerts {
		sloRef, ok := sloRefs[alert.SLOName]
		if !ok {
			return fmt.Errorf("alert %s references unknown SLO %s", alert.ID, alert.SLOName)
		}
		if err := client.ApplyAlert(ctx, ApplyAlertRequest{
			Project: plan.Project,
			SLOName: alert.SLOName,
			SLORef:  sloRef,
			Alert:   alert,
			Labels:  alert.Labels,
		}); err != nil {
			return fmt.Errorf("apply alert %s: %w", alert.ID, err)
		}
	}

	if err := client.ApplyDashboard(ctx, ApplyDashboardRequest{
		Project:   plan.Project,
		Dashboard: plan.Dashboard,
		Labels:    plan.Dashboard.Labels,
	}); err != nil {
		return fmt.Errorf("apply dashboard: %w", err)
	}

	return nil
}

func DeletePlan(ctx context.Context, client Client, plan planner.Plan) error {
	return client.DeleteManagedResources(ctx, DeleteRequest{
		Project:   plan.Project,
		ServiceID: plan.ServiceID,
		Labels:    plan.Dashboard.Labels,
	})
}
