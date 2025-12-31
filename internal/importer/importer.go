package importer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/bayneri/margin/internal/monitoring"
	"github.com/bayneri/margin/internal/spec"
	"google.golang.org/genproto/googleapis/type/calendarperiod"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Options struct {
	Project     string
	ServiceID   string
	ServiceType string
}

type Result struct {
	Spec     spec.Spec
	Warnings []string
}

func Import(ctx context.Context, client *monitoring.GCPClient, opts Options) (Result, error) {
	if strings.TrimSpace(opts.Project) == "" {
		return Result{}, errors.New("--project is required")
	}
	if strings.TrimSpace(opts.ServiceID) == "" {
		return Result{}, errors.New("--service is required")
	}

	slos, err := client.ListServiceLevelObjectives(ctx, opts.Project, opts.ServiceID)
	if err != nil {
		return Result{}, err
	}
	if len(slos) == 0 {
		return Result{}, fmt.Errorf("no SLOs found for service %q", opts.ServiceID)
	}

	serviceType := strings.TrimSpace(opts.ServiceType)
	if serviceType == "" {
		serviceType = inferServiceType(slos)
		if serviceType == "" {
			return Result{}, errors.New("unable to infer service type; set --service-type")
		}
	}
	if _, err := spec.TemplateForService(serviceType); err != nil {
		return Result{}, err
	}

	specDoc := spec.Spec{
		APIVersion: spec.APIVersionV1,
		Kind:       spec.KindServiceSLO,
		Metadata: spec.Metadata{
			Name:    opts.ServiceID,
			Service: serviceType,
			Project: opts.Project,
			Labels:  commonLabels(slos),
		},
	}
	var warnings []string

	sort.Slice(slos, func(i, j int) bool {
		if slos[i].GetDisplayName() == slos[j].GetDisplayName() {
			return slos[i].GetName() < slos[j].GetName()
		}
		return slos[i].GetDisplayName() < slos[j].GetDisplayName()
	})

	for _, slo := range slos {
		out, warn, ok := sloToSpec(slo, serviceType)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if ok {
			specDoc.SLOs = append(specDoc.SLOs, out)
		}
	}

	if len(specDoc.SLOs) == 0 {
		return Result{}, errors.New("no supported SLOs found to import")
	}

	if labelWarn := labelWarnings(slos); labelWarn != "" {
		warnings = append(warnings, labelWarn)
	}

	return Result{Spec: specDoc, Warnings: warnings}, nil
}

func inferServiceType(slos []*monitoringpb.ServiceLevelObjective) string {
	counts := map[string]int{}
	for _, slo := range slos {
		metrics := metricsFromSLO(slo)
		for _, tpl := range knownTemplates() {
			for _, metric := range metrics {
				if _, ok := tpl.Metrics[metric]; ok {
					counts[tpl.Name]++
				}
			}
		}
	}

	type candidate struct {
		name  string
		count int
	}
	var candidates []candidate
	for name, count := range counts {
		candidates = append(candidates, candidate{name: name, count: count})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].count > candidates[j].count
	})
	if len(candidates) == 0 || candidates[0].count == 0 {
		return ""
	}
	return candidates[0].name
}

func metricsFromSLO(slo *monitoringpb.ServiceLevelObjective) []string {
	var metrics []string
	sli := slo.GetServiceLevelIndicator()
	if sli == nil {
		return metrics
	}
	rb := sli.GetRequestBased()
	if rb == nil {
		return metrics
	}
	if gtr := rb.GetGoodTotalRatio(); gtr != nil {
		if metric, _, _ := parseFilter(gtr.GetGoodServiceFilter()); metric != "" {
			metrics = append(metrics, metric)
		}
		if metric, _, _ := parseFilter(gtr.GetTotalServiceFilter()); metric != "" {
			metrics = append(metrics, metric)
		}
	}
	if cut := rb.GetDistributionCut(); cut != nil {
		if metric, _, _ := parseFilter(cut.GetDistributionFilter()); metric != "" {
			metrics = append(metrics, metric)
		}
	}
	return metrics
}

func knownTemplates() []spec.ServiceTemplate {
	var templates []spec.ServiceTemplate
	for _, name := range []string{
		"cloud-run",
		"https-load-balancer",
		"gke-ingress",
		"cloud-sql",
		"gke-service",
		"gke-gateway",
		"gce-lb",
		"cloud-functions",
		"pubsub-subscription",
		"cloud-storage",
		"cloud-tasks",
		"bigquery",
		"spanner",
		"cloud-cdn",
		"gce-uptime",
	} {
		if tpl, err := spec.TemplateForService(name); err == nil {
			templates = append(templates, tpl)
		}
	}
	return templates
}

func sloToSpec(slo *monitoringpb.ServiceLevelObjective, serviceType string) (spec.SLO, string, bool) {
	id := lastSegment(slo.GetName())
	if id == "" {
		id = sanitizeName(slo.GetDisplayName())
	}
	if id == "" {
		return spec.SLO{}, fmt.Sprintf("skipping SLO %q: missing id", slo.GetName()), false
	}

	window, period, warn := sloPeriod(slo)
	if warn != "" {
		return spec.SLO{}, warn, false
	}

	out := spec.SLO{
		Name:      id,
		Objective: roundPercent(slo.GetGoal() * 100),
		Window:    window,
		Period:    period,
	}

	sli := slo.GetServiceLevelIndicator()
	if sli == nil {
		return spec.SLO{}, fmt.Sprintf("skipping %s: missing SLI", id), false
	}
	template, _ := spec.TemplateForService(serviceType)

	if wb := sli.GetWindowsBased(); wb != nil {
		if gtr := wb.GetGoodTotalRatioThreshold(); gtr != nil {
			perf := gtr.GetPerformance()
			if perf == nil {
				return spec.SLO{}, fmt.Sprintf("skipping %s: windows-based SLI missing performance block", id), false
			}
			if gtr.GetBasicSliPerformance() != nil {
				return spec.SLO{}, fmt.Sprintf("skipping %s: windows-based basic SLI not supported (no explicit metrics)", id), false
			}
			if gtrRatio := perf.GetGoodTotalRatio(); gtrRatio != nil {
				goodMetric, _, goodExtra := parseFilter(gtrRatio.GetGoodServiceFilter())
				badMetric, _, badExtra := parseFilter(gtrRatio.GetBadServiceFilter())
				totalMetric, _, totalExtra := parseFilter(gtrRatio.GetTotalServiceFilter())

				if goodMetric == "" && badMetric != "" && totalMetric != "" {
					goodMetric = totalMetric
					goodExtra = combineFilters(totalExtra, negateFilter(badExtra))
				}

				if goodMetric == "" || totalMetric == "" {
					return spec.SLO{}, fmt.Sprintf("skipping %s: unable to parse windows-based filters", id), false
				}
				out.SLI = spec.SLI{
					Type: "request-based",
					Good: &spec.MetricDef{
						Metric: goodMetric,
						Filter: goodExtra,
					},
					Total: &spec.MetricDef{
						Metric: totalMetric,
						Filter: totalExtra,
					},
				}
				return out, "converted windows-based SLI to request-based good/total", true
			}
			if cut := perf.GetDistributionCut(); cut != nil {
				metric, _, extra := parseFilter(cut.GetDistributionFilter())
				if metric == "" {
					return spec.SLO{}, fmt.Sprintf("skipping %s: unable to parse windows-based latency filter", id), false
				}
				out.SLI = spec.SLI{
					Type:      "latency",
					Metric:    metric,
					Filter:    extra,
					Threshold: formatSeconds(cut.GetRange().GetMax()),
				}
				return out, "converted windows-based distribution cut to latency threshold", true
			}
		}
		return spec.SLO{}, fmt.Sprintf("skipping %s: windows-based SLI not yet supported (criteria: %T)", id, wb.GetWindowCriterion()), false
	}
	if basic := sli.GetBasicSli(); basic != nil {
		if lat := basic.GetLatency(); lat != nil {
			metric := defaultLatencyMetric(template)
			if metric == "" {
				return spec.SLO{}, fmt.Sprintf("skipping %s: cannot map basic latency (no template latency metric)", id), false
			}
			threshold := lat.GetThreshold()
			if threshold == nil {
				return spec.SLO{}, fmt.Sprintf("skipping %s: basic latency missing threshold", id), false
			}
			out.SLI = spec.SLI{
				Type:      "latency",
				Metric:    metric,
				Threshold: formatSeconds(threshold.AsDuration().Seconds()),
				Filter:    fmt.Sprintf(`resource.type="%s"`, template.ResourceType),
			}
			return out, "converted basic latency SLI to latency threshold", true
		}
		if basic.GetAvailability() != nil {
			requestMetric := defaultRequestMetric(template)
			if requestMetric == "" {
				return spec.SLO{}, fmt.Sprintf("skipping %s: cannot map basic availability (no template request metric)", id), false
			}
			out.SLI = spec.SLI{
				Type: "request-based",
				Good: &spec.MetricDef{
					Metric: requestMetric,
					Filter: fmt.Sprintf(`resource.type="%s"`, template.ResourceType),
				},
				Total: &spec.MetricDef{
					Metric: requestMetric,
					Filter: fmt.Sprintf(`resource.type="%s"`, template.ResourceType),
				},
			}
			return out, "converted basic availability SLI to request-based good/total", true
		}
		return spec.SLO{}, fmt.Sprintf("skipping %s: basic SLI missing latency/availability", id), false
	}
	rb := sli.GetRequestBased()
	if rb == nil {
		return spec.SLO{}, fmt.Sprintf("skipping %s: unsupported SLI type %T", id, sli.GetType()), false
	}

	if gtr := rb.GetGoodTotalRatio(); gtr != nil {
		goodMetric, _, goodExtra := parseFilter(gtr.GetGoodServiceFilter())
		badMetric, _, badExtra := parseFilter(gtr.GetBadServiceFilter())
		totalMetric, _, totalExtra := parseFilter(gtr.GetTotalServiceFilter())

		if goodMetric == "" && badMetric != "" && totalMetric != "" {
			goodMetric = totalMetric
			goodExtra = combineFilters(totalExtra, negateFilter(badExtra))
		}

		if goodMetric == "" || totalMetric == "" {
			return spec.SLO{}, fmt.Sprintf("skipping %s: unable to parse request-based filters", id), false
		}
		out.SLI = spec.SLI{
			Type: "request-based",
			Good: &spec.MetricDef{
				Metric: goodMetric,
				Filter: goodExtra,
			},
			Total: &spec.MetricDef{
				Metric: totalMetric,
				Filter: totalExtra,
			},
		}
		return out, "", true
	}

	if cut := rb.GetDistributionCut(); cut != nil {
		metric, _, extra := parseFilter(cut.GetDistributionFilter())
		if metric == "" {
			return spec.SLO{}, fmt.Sprintf("skipping %s: unable to parse latency filter", id), false
		}
		out.SLI = spec.SLI{
			Type:      "latency",
			Metric:    metric,
			Filter:    extra,
			Threshold: formatSeconds(cut.GetRange().GetMax()),
		}
		return out, "", true
	}

	return spec.SLO{}, fmt.Sprintf("skipping %s: unsupported SLI method", id), false
}

func sloPeriod(slo *monitoringpb.ServiceLevelObjective) (string, string, string) {
	if rolling := slo.GetRollingPeriod(); rolling != nil {
		window, err := durationToWindow(rolling)
		if err != nil {
			return "", "", fmt.Sprintf("skipping %s: %v", slo.GetDisplayName(), err)
		}
		return window, "rolling", ""
	}
	if cal := slo.GetCalendarPeriod(); cal != 0 {
		window := calendarToWindow(cal)
		if window == "" {
			return "", "", fmt.Sprintf("skipping %s: unsupported calendar period %v", slo.GetDisplayName(), cal)
		}
		return window, "calendar", ""
	}
	return "", "", fmt.Sprintf("skipping %s: missing period", slo.GetDisplayName())
}

func durationToWindow(duration *durationpb.Duration) (string, error) {
	if duration == nil {
		return "", errors.New("missing rolling period")
	}
	value := duration.AsDuration()
	switch {
	case value%(7*24*time.Hour) == 0:
		return fmt.Sprintf("%dw", value/(7*24*time.Hour)), nil
	case value%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", value/(24*time.Hour)), nil
	case value%time.Hour == 0:
		return fmt.Sprintf("%dh", value/time.Hour), nil
	case value%time.Minute == 0:
		return fmt.Sprintf("%dm", value/time.Minute), nil
	default:
		return "", fmt.Errorf("rolling period %s cannot be expressed as window", value)
	}
}

func calendarToWindow(period calendarperiod.CalendarPeriod) string {
	switch period {
	case calendarperiod.CalendarPeriod_DAY:
		return "1d"
	case calendarperiod.CalendarPeriod_WEEK:
		return "1w"
	case calendarperiod.CalendarPeriod_FORTNIGHT:
		return "2w"
	case calendarperiod.CalendarPeriod_MONTH:
		return "30d"
	default:
		return ""
	}
}

func parseFilter(filter string) (string, string, string) {
	parts := strings.Split(filter, " AND ")
	var metricType, resourceType string
	var extra []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if key, value, ok := parseFilterPart(trimmed); ok {
			switch key {
			case "metric.type":
				metricType = value
				continue
			case "resource.type":
				resourceType = value
				continue
			}
		}
		extra = append(extra, trimmed)
	}
	return metricType, resourceType, strings.Join(extra, " AND ")
}

func parseFilterPart(part string) (string, string, bool) {
	segments := strings.SplitN(part, "=", 2)
	if len(segments) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(segments[0])
	value := strings.TrimSpace(segments[1])
	value = strings.Trim(value, "\"")
	if key == "metric.type" || key == "resource.type" {
		return key, value, true
	}
	return "", "", false
}

func formatSeconds(seconds float64) string {
	duration := durationpb.New(secondsToDuration(seconds)).AsDuration()
	switch {
	case duration%time.Millisecond == 0 && duration < time.Second:
		return fmt.Sprintf("%dms", duration/time.Millisecond)
	case duration%time.Second == 0:
		return fmt.Sprintf("%ds", duration/time.Second)
	default:
		return duration.String()
	}
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

func roundPercent(value float64) float64 {
	return float64(int(value*10000+0.5)) / 10000
}

func lastSegment(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func sanitizeName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return ""
	}
	var out []rune
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '-')
		}
	}
	return strings.Trim(string(out), "-")
}

func defaultRequestMetric(template spec.ServiceTemplate) string {
	for name := range template.Metrics {
		if strings.Contains(name, "request_count") || strings.Contains(name, "request") {
			return name
		}
	}
	for name := range template.Metrics {
		return name
	}
	return ""
}

func defaultLatencyMetric(template spec.ServiceTemplate) string {
	for name := range template.Metrics {
		if strings.Contains(name, "latency") {
			return name
		}
	}
	return ""
}

func commonLabels(slos []*monitoringpb.ServiceLevelObjective) map[string]string {
	if len(slos) == 0 {
		return map[string]string{}
	}
	labels := slos[0].GetUserLabels()
	if labels == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for k, v := range labels {
		out[k] = v
	}
	for _, slo := range slos[1:] {
		if !labelsEqual(out, slo.GetUserLabels()) {
			return map[string]string{}
		}
	}
	return out
}

func labelWarnings(slos []*monitoringpb.ServiceLevelObjective) string {
	if len(slos) == 0 {
		return ""
	}
	base := slos[0].GetUserLabels()
	for _, slo := range slos[1:] {
		if !labelsEqual(base, slo.GetUserLabels()) {
			return "SLO user_labels differ; metadata.labels omitted"
		}
	}
	return ""
}

func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func negateFilter(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return ""
	}
	return fmt.Sprintf("NOT (%s)", filter)
}

func combineFilters(parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(part))
		}
	}
	return strings.Join(nonEmpty, " AND ")
}
