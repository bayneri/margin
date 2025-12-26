package monitoring

import "context"

type Client interface {
	ApplySLO(ctx context.Context, req ApplySLORequest) error
	ApplyAlert(ctx context.Context, req ApplyAlertRequest) error
	ApplyDashboard(ctx context.Context, req ApplyDashboardRequest) error
	DeleteManagedResources(ctx context.Context, req DeleteRequest) error
}

type ApplySLORequest struct {
	Project string
	Payload interface{}
}

type ApplyAlertRequest struct {
	Project string
	Payload interface{}
}

type ApplyDashboardRequest struct {
	Project string
	Payload interface{}
}

type DeleteRequest struct {
	Project string
}
