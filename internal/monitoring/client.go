package monitoring

import (
	"context"

	"github.com/bayneri/margin/internal/planner"
	"github.com/bayneri/margin/internal/spec"
)

type Client interface {
	EnsureService(ctx context.Context, req EnsureServiceRequest) error
	ApplySLO(ctx context.Context, req ApplySLORequest) (string, error)
	ApplyAlert(ctx context.Context, req ApplyAlertRequest) error
	ApplyDashboard(ctx context.Context, req ApplyDashboardRequest) error
	DeleteManagedResources(ctx context.Context, req DeleteRequest) error
}

type EnsureServiceRequest struct {
	Project     string
	ServiceID   string
	DisplayName string
	Labels      map[string]string
}

type ApplySLORequest struct {
	Project   string
	ServiceID string
	SLO       planner.SLOPlan
	Template  spec.ServiceTemplate
	Labels    map[string]string
}

type ApplyAlertRequest struct {
	Project string
	SLOName string
	SLORef  string
	Alert   planner.AlertPlan
	Labels  map[string]string
}

type ApplyDashboardRequest struct {
	Project   string
	ServiceID string
	Dashboard planner.DashboardPlan
	SLOs      []planner.SLOPlan
	Template  spec.ServiceTemplate
	Labels    map[string]string
}

type DeleteRequest struct {
	Project   string
	ServiceID string
	Labels    map[string]string
}
