package monitoring

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	dashboard "cloud.google.com/go/monitoring/dashboard/apiv1"
	"cloud.google.com/go/monitoring/dashboard/apiv1/dashboardpb"
	"github.com/bayneri/margin/internal/planner"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/type/calendarperiod"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type GCPClient struct {
	serviceClient *monitoring.ServiceMonitoringClient
	alertClient   *monitoring.AlertPolicyClient
	dashClient    *dashboard.DashboardsClient
}

func NewGCPClient(ctx context.Context) (*GCPClient, error) {
	serviceClient, err := monitoring.NewServiceMonitoringClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create service monitoring client: %w", err)
	}
	alertClient, err := monitoring.NewAlertPolicyClient(ctx)
	if err != nil {
		serviceClient.Close()
		return nil, fmt.Errorf("create alert policy client: %w", err)
	}
	dashClient, err := dashboard.NewDashboardsClient(ctx)
	if err != nil {
		serviceClient.Close()
		alertClient.Close()
		return nil, fmt.Errorf("create dashboards client: %w", err)
	}

	return &GCPClient{
		serviceClient: serviceClient,
		alertClient:   alertClient,
		dashClient:    dashClient,
	}, nil
}

func (c *GCPClient) Close() error {
	var errs []string
	if err := c.serviceClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := c.alertClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := c.dashClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("close clients: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (c *GCPClient) EnsureService(ctx context.Context, req EnsureServiceRequest) error {
	serviceName := fmt.Sprintf("projects/%s/services/%s", req.Project, req.ServiceID)
	_, err := c.serviceClient.GetService(ctx, &monitoringpb.GetServiceRequest{Name: serviceName})
	if err == nil {
		_, err = c.serviceClient.UpdateService(ctx, &monitoringpb.UpdateServiceRequest{
			Service: &monitoringpb.Service{
				Name:        serviceName,
				DisplayName: req.DisplayName,
				UserLabels:  req.Labels,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name", "user_labels"}},
		})
		return err
	}
	if status.Code(err) != codes.NotFound {
		return err
	}

	_, err = c.serviceClient.CreateService(ctx, &monitoringpb.CreateServiceRequest{
		Parent:    fmt.Sprintf("projects/%s", req.Project),
		ServiceId: req.ServiceID,
		Service: &monitoringpb.Service{
			DisplayName: req.DisplayName,
			UserLabels:  req.Labels,
			Identifier: &monitoringpb.Service_Custom_{
				Custom: &monitoringpb.Service_Custom{},
			},
		},
	})
	return err
}

func (c *GCPClient) ApplySLO(ctx context.Context, req ApplySLORequest) (string, error) {
	serviceName := fmt.Sprintf("projects/%s/services/%s", req.Project, req.ServiceID)
	desired, err := buildSLO(req)
	if err != nil {
		return "", err
	}

	existing, err := c.findSLO(ctx, serviceName, req.SLO.DisplayName)
	if err != nil {
		return "", err
	}
	if existing != nil {
		desired.Name = existing.Name
		updated, err := c.serviceClient.UpdateServiceLevelObjective(ctx, &monitoringpb.UpdateServiceLevelObjectiveRequest{
			ServiceLevelObjective: desired,
			UpdateMask:            sloUpdateMask(req.SLO.Period),
		})
		if err != nil {
			return "", err
		}
		return updated.Name, nil
	}

	created, err := c.serviceClient.CreateServiceLevelObjective(ctx, &monitoringpb.CreateServiceLevelObjectiveRequest{
		Parent:                  serviceName,
		ServiceLevelObjectiveId: req.SLO.ResourceID,
		ServiceLevelObjective:   desired,
	})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

func (c *GCPClient) ApplyAlert(ctx context.Context, req ApplyAlertRequest) error {
	policy, err := buildAlertPolicy(req)
	if err != nil {
		return err
	}

	existing, err := c.findAlertPolicy(ctx, req.Project, req.Alert.DisplayName)
	if err != nil {
		return err
	}
	if existing != nil {
		policy.Name = existing.Name
		_, err = c.alertClient.UpdateAlertPolicy(ctx, &monitoringpb.UpdateAlertPolicyRequest{
			AlertPolicy: policy,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{
				"display_name",
				"documentation",
				"conditions",
				"combiner",
				"user_labels",
				"enabled",
				"severity",
			}},
		})
		return err
	}

	_, err = c.alertClient.CreateAlertPolicy(ctx, &monitoringpb.CreateAlertPolicyRequest{
		Name:        fmt.Sprintf("projects/%s", req.Project),
		AlertPolicy: policy,
	})
	return err
}

func (c *GCPClient) ApplyDashboard(ctx context.Context, req ApplyDashboardRequest) error {
	dashboard := buildDashboard(req)

	existing, err := c.findDashboard(ctx, req.Project, req.Dashboard.DisplayName)
	if err != nil {
		return err
	}
	if existing != nil {
		dashboard.Name = existing.Name
		dashboard.Etag = existing.Etag
		_, err = c.dashClient.UpdateDashboard(ctx, &dashboardpb.UpdateDashboardRequest{
			Dashboard: dashboard,
		})
		return err
	}

	_, err = c.dashClient.CreateDashboard(ctx, &dashboardpb.CreateDashboardRequest{
		Parent:    fmt.Sprintf("projects/%s", req.Project),
		Dashboard: dashboard,
	})
	return err
}

func (c *GCPClient) DeleteManagedResources(ctx context.Context, req DeleteRequest) error {
	serviceName := fmt.Sprintf("projects/%s/services/%s", req.Project, req.ServiceID)

	sloIter := c.serviceClient.ListServiceLevelObjectives(ctx, &monitoringpb.ListServiceLevelObjectivesRequest{Parent: serviceName})
	for {
		slo, err := sloIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if !hasManagedLabel(slo.UserLabels, req.Labels) {
			continue
		}
		if err := c.serviceClient.DeleteServiceLevelObjective(ctx, &monitoringpb.DeleteServiceLevelObjectiveRequest{Name: slo.Name}); err != nil {
			return err
		}
	}

	alertIter := c.alertClient.ListAlertPolicies(ctx, &monitoringpb.ListAlertPoliciesRequest{Name: fmt.Sprintf("projects/%s", req.Project)})
	for {
		policy, err := alertIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if !hasManagedLabel(policy.UserLabels, req.Labels) {
			continue
		}
		if err := c.alertClient.DeleteAlertPolicy(ctx, &monitoringpb.DeleteAlertPolicyRequest{Name: policy.Name}); err != nil {
			return err
		}
	}

	dashIter := c.dashClient.ListDashboards(ctx, &dashboardpb.ListDashboardsRequest{Parent: fmt.Sprintf("projects/%s", req.Project)})
	for {
		dashboard, err := dashIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if !hasManagedLabel(dashboard.Labels, req.Labels) {
			continue
		}
		if err := c.dashClient.DeleteDashboard(ctx, &dashboardpb.DeleteDashboardRequest{Name: dashboard.Name}); err != nil {
			return err
		}
	}

	return nil
}

func buildSLO(req ApplySLORequest) (*monitoringpb.ServiceLevelObjective, error) {
	indicator, err := buildIndicator(req)
	if err != nil {
		return nil, err
	}
	periodKind, rolling, calendar, err := buildPeriod(req.SLO.Period, req.SLO.Window)
	if err != nil {
		return nil, err
	}
	goal := roundGoal(req.SLO.Objective / 100)

	slo := &monitoringpb.ServiceLevelObjective{
		DisplayName:           req.SLO.DisplayName,
		ServiceLevelIndicator: indicator,
		Goal:                  goal,
		UserLabels:            req.Labels,
	}
	if periodKind == "calendar" {
		slo.Period = &monitoringpb.ServiceLevelObjective_CalendarPeriod{
			CalendarPeriod: calendar,
		}
	} else {
		slo.Period = &monitoringpb.ServiceLevelObjective_RollingPeriod{
			RollingPeriod: durationpb.New(rolling),
		}
	}
	return slo, nil
}

func buildIndicator(req ApplySLORequest) (*monitoringpb.ServiceLevelIndicator, error) {
	resourceType := req.Template.ResourceType
	if resourceType == "" {
		return nil, fmt.Errorf("service template missing resource type")
	}

	switch req.SLO.SLI.Type {
	case "request-based":
		goodFilter := buildFilter(req.SLO.SLI.Good.Metric, resourceType, req.SLO.SLI.Good.Filter)
		totalFilter := buildFilter(req.SLO.SLI.Total.Metric, resourceType, req.SLO.SLI.Total.Filter)
		ratio := &monitoringpb.TimeSeriesRatio{
			GoodServiceFilter:  goodFilter,
			TotalServiceFilter: totalFilter,
		}
		return &monitoringpb.ServiceLevelIndicator{
			Type: &monitoringpb.ServiceLevelIndicator_RequestBased{
				RequestBased: &monitoringpb.RequestBasedSli{
					Method: &monitoringpb.RequestBasedSli_GoodTotalRatio{GoodTotalRatio: ratio},
				},
			},
		}, nil
	case "latency":
		threshold, err := parseThreshold(req.SLO.SLI.Threshold)
		if err != nil {
			return nil, err
		}
		filter := buildFilter(req.SLO.SLI.Metric, resourceType, req.SLO.SLI.Filter)
		cut := &monitoringpb.DistributionCut{
			DistributionFilter: filter,
			Range: &monitoringpb.Range{
				Min: 0,
				Max: threshold,
			},
		}
		return &monitoringpb.ServiceLevelIndicator{
			Type: &monitoringpb.ServiceLevelIndicator_RequestBased{
				RequestBased: &monitoringpb.RequestBasedSli{
					Method: &monitoringpb.RequestBasedSli_DistributionCut{DistributionCut: cut},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported SLI type %q", req.SLO.SLI.Type)
	}
}

func buildAlertPolicy(req ApplyAlertRequest) (*monitoringpb.AlertPolicy, error) {
	var conditions []*monitoringpb.AlertPolicy_Condition
	for _, window := range req.Alert.Windows {
		windowDuration, err := parseWindow(window)
		if err != nil {
			return nil, err
		}
		condition := &monitoringpb.AlertPolicy_Condition{
			DisplayName: fmt.Sprintf("%s %s", req.Alert.DisplayName, window),
			Condition: &monitoringpb.AlertPolicy_Condition_ConditionThreshold{
				ConditionThreshold: &monitoringpb.AlertPolicy_Condition_MetricThreshold{
					Filter:                buildBurnRateFilter(req.SLORef, window),
					Comparison:            monitoringpb.ComparisonType_COMPARISON_GT,
					ThresholdValue:        req.Alert.BurnRate,
					Duration:              durationpb.New(windowDuration),
					EvaluationMissingData: monitoringpb.AlertPolicy_Condition_EVALUATION_MISSING_DATA_NO_OP,
				},
			},
		}
		conditions = append(conditions, condition)
	}

	doc := buildAlertDocumentation(req.Alert, req.SLOName)

	return &monitoringpb.AlertPolicy{
		DisplayName: req.Alert.DisplayName,
		Documentation: &monitoringpb.AlertPolicy_Documentation{
			Content:  doc,
			MimeType: "text/markdown",
		},
		Conditions: conditions,
		Combiner:   monitoringpb.AlertPolicy_AND,
		UserLabels: req.Labels,
		Enabled:    wrapperspb.Bool(true),
		Severity:   severityFor(req.Alert.Severity),
	}, nil
}

func buildDashboard(req ApplyDashboardRequest) *dashboardpb.Dashboard {
	content := fmt.Sprintf("# %s\n\nGenerated by margin. Use this dashboard as a starting point for SLO and alert review.", req.Dashboard.Service)
	widget := &dashboardpb.Widget{
		Title: "Service overview",
		Content: &dashboardpb.Widget_Text{
			Text: &dashboardpb.Text{
				Content: content,
				Format:  dashboardpb.Text_MARKDOWN,
			},
		},
	}

	return &dashboardpb.Dashboard{
		DisplayName: req.Dashboard.DisplayName,
		Labels:      req.Labels,
		Layout: &dashboardpb.Dashboard_GridLayout{
			GridLayout: &dashboardpb.GridLayout{
				Columns: 2,
				Widgets: []*dashboardpb.Widget{widget},
			},
		},
	}
}

func (c *GCPClient) findSLO(ctx context.Context, serviceName, displayName string) (*monitoringpb.ServiceLevelObjective, error) {
	iter := c.serviceClient.ListServiceLevelObjectives(ctx, &monitoringpb.ListServiceLevelObjectivesRequest{Parent: serviceName})
	for {
		slo, err := iter.Next()
		if err == iterator.Done {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if slo.DisplayName == displayName {
			return slo, nil
		}
	}
}

func (c *GCPClient) findAlertPolicy(ctx context.Context, project, displayName string) (*monitoringpb.AlertPolicy, error) {
	iter := c.alertClient.ListAlertPolicies(ctx, &monitoringpb.ListAlertPoliciesRequest{Name: fmt.Sprintf("projects/%s", project)})
	for {
		policy, err := iter.Next()
		if err == iterator.Done {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if policy.DisplayName == displayName {
			return policy, nil
		}
	}
}

func (c *GCPClient) findDashboard(ctx context.Context, project, displayName string) (*dashboardpb.Dashboard, error) {
	iter := c.dashClient.ListDashboards(ctx, &dashboardpb.ListDashboardsRequest{Parent: fmt.Sprintf("projects/%s", project)})
	for {
		dashboard, err := iter.Next()
		if err == iterator.Done {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if dashboard.DisplayName == displayName {
			return dashboard, nil
		}
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

func severityFor(value string) monitoringpb.AlertPolicy_Severity {
	switch strings.ToLower(value) {
	case "page":
		return monitoringpb.AlertPolicy_CRITICAL
	case "ticket":
		return monitoringpb.AlertPolicy_WARNING
	default:
		return monitoringpb.AlertPolicy_SEVERITY_UNSPECIFIED
	}
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

func parseWindow(window string) (time.Duration, error) {
	if window == "" {
		return 0, fmt.Errorf("window is empty")
	}
	unit := window[len(window)-1]
	value := window[:len(window)-1]
	amount, err := strconv.Atoi(value)
	if err != nil {
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

func hasManagedLabel(labels map[string]string, filter map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	for key, value := range filter {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func roundGoal(goal float64) float64 {
	const precision = 10000.0
	return math.Round(goal*precision) / precision
}

func buildPeriod(period, window string) (string, time.Duration, calendarperiod.CalendarPeriod, error) {
	switch strings.TrimSpace(period) {
	case "", "rolling":
		rolling, err := parseWindow(window)
		if err != nil {
			return "", 0, calendarperiod.CalendarPeriod(0), err
		}
		return "rolling", rolling, calendarperiod.CalendarPeriod(0), nil
	case "calendar":
		cal, err := parseCalendarWindow(window)
		if err != nil {
			return "", 0, calendarperiod.CalendarPeriod(0), err
		}
		return "calendar", 0, cal, nil
	default:
		return "", 0, calendarperiod.CalendarPeriod(0), fmt.Errorf("unknown period %q", period)
	}
}

func parseCalendarWindow(window string) (calendarperiod.CalendarPeriod, error) {
	switch window {
	case "1d":
		return calendarperiod.CalendarPeriod_DAY, nil
	case "1w":
		return calendarperiod.CalendarPeriod_WEEK, nil
	case "2w":
		return calendarperiod.CalendarPeriod_FORTNIGHT, nil
	case "30d":
		return calendarperiod.CalendarPeriod_MONTH, nil
	default:
		return calendarperiod.CalendarPeriod(0), fmt.Errorf("calendar window must be 1d, 1w, 2w, or 30d")
	}
}

func sloUpdateMask(period string) *fieldmaskpb.FieldMask {
	paths := []string{
		"display_name",
		"goal",
		"service_level_indicator",
		"user_labels",
	}
	if strings.TrimSpace(period) == "calendar" {
		paths = append(paths, "calendar_period")
	} else {
		paths = append(paths, "rolling_period")
	}
	return &fieldmaskpb.FieldMask{Paths: paths}
}
