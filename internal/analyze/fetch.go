package analyze

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GCPReader struct {
	serviceClient *monitoring.ServiceMonitoringClient
	metricClient  *monitoring.MetricClient
}

func NewGCPReader(ctx context.Context) (*GCPReader, error) {
	serviceClient, err := monitoring.NewServiceMonitoringClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create service monitoring client: %w", err)
	}
	metricClient, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		serviceClient.Close()
		return nil, fmt.Errorf("create metric client: %w", err)
	}
	return &GCPReader{serviceClient: serviceClient, metricClient: metricClient}, nil
}

func (r *GCPReader) Close() error {
	var errs []string
	if err := r.serviceClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := r.metricClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("close monitoring clients: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *GCPReader) ListServiceLevelObjectives(ctx context.Context, serviceName string, max int) ([]SLO, error) {
	iter := r.serviceClient.ListServiceLevelObjectives(ctx, &monitoringpb.ListServiceLevelObjectivesRequest{Parent: serviceName})
	var out []SLO
	for {
		slo, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		converted, err := toAnalyzeSLO(slo)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out, nil
}

func (r *GCPReader) FetchCompliance(ctx context.Context, project string, sloName string, start, end time.Time) (float64, error) {
	interval := &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}
	window := end.Sub(start)
	filter := fmt.Sprintf("select_slo_compliance(%q)", sloName)
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:     fmt.Sprintf("projects/%s", project),
		Filter:   filter,
		Interval: interval,
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(window),
			PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_MEAN,
			CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_MEAN,
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}

	iter := r.metricClient.ListTimeSeries(ctx, req)
	for {
		ts, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}
		if len(ts.Points) == 0 {
			continue
		}
		point := ts.Points[0]
		if point.Value == nil {
			continue
		}
		switch v := point.Value.GetValue().(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			return v.DoubleValue, nil
		case *monitoringpb.TypedValue_Int64Value:
			return float64(v.Int64Value), nil
		default:
			return point.Value.GetDoubleValue(), nil
		}
	}

	return 0, status.Error(codes.NotFound, "no compliance points returned")
}

func toAnalyzeSLO(slo *monitoringpb.ServiceLevelObjective) (SLO, error) {
	result := SLO{
		Name:        slo.GetName(),
		DisplayName: slo.GetDisplayName(),
		Goal:        slo.GetGoal(),
	}
	if slo.GetRollingPeriod() != nil {
		result.RollingDays = int64(slo.GetRollingPeriod().AsDuration().Hours() / 24)
	}
	if slo.GetCalendarPeriod() != 0 {
		value := slo.GetCalendarPeriod().String()
		result.Calendar = &value
	}
	indicator := slo.GetServiceLevelIndicator()
	if indicator == nil {
		return result, nil
	}
	switch indicator.GetType().(type) {
	case *monitoringpb.ServiceLevelIndicator_RequestBased:
		result.SLIType = "request-based"
		method := indicator.GetRequestBased().GetMethod()
		switch method.(type) {
		case *monitoringpb.RequestBasedSli_GoodTotalRatio:
			result.SLIMethod = "good-total-ratio"
		case *monitoringpb.RequestBasedSli_DistributionCut:
			result.SLIMethod = "distribution-cut"
		default:
			result.SLIMethod = "unknown"
		}
	case *monitoringpb.ServiceLevelIndicator_WindowsBased:
		result.SLIType = "windows-based"
	case *monitoringpb.ServiceLevelIndicator_BasicSli:
		result.SLIType = "basic-sli"
	default:
		result.SLIType = "unknown"
	}
	return result, nil
}
