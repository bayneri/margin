package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	APIVersionV1   = "margin/v1"
	KindServiceSLO = "ServiceSLO"
)

type Spec struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Alerting   Alerting `yaml:"alerting"`
	SLOs       []SLO    `yaml:"slos"`
}

type Metadata struct {
	Name    string            `yaml:"name"`
	Service string            `yaml:"service"`
	Project string            `yaml:"project"`
	Labels  map[string]string `yaml:"labels"`
	Runbook string            `yaml:"runbook"`
}

type Alerting struct {
	BurnRateResourceType string `yaml:"burnRateResourceType"`
}

type SLO struct {
	Name      string      `yaml:"name"`
	Objective float64     `yaml:"objective"`
	Window    string      `yaml:"window"`
	Period    string      `yaml:"period"`
	SLI       SLI         `yaml:"sli"`
	Alerting  SLOAlerting `yaml:"alerting"`
}

type SLI struct {
	Type      string     `yaml:"type"`
	Good      *MetricDef `yaml:"good"`
	Total     *MetricDef `yaml:"total"`
	Metric    string     `yaml:"metric"`
	Filter    string     `yaml:"filter"`
	Threshold string     `yaml:"threshold"`
}

type MetricDef struct {
	Metric string `yaml:"metric"`
	Filter string `yaml:"filter"`
}

type SLOAlerting struct {
	Fast *AlertOverride `yaml:"fast"`
	Slow *AlertOverride `yaml:"slow"`
}

type AlertOverride struct {
	Windows  []string `yaml:"windows"`
	BurnRate float64  `yaml:"burnRate"`
}

var windowRe = regexp.MustCompile(`^(\d+)([smhdw])$`)

func (s Spec) Validate() error {
	var errs []string
	if s.APIVersion != APIVersionV1 {
		errs = append(errs, fmt.Sprintf("apiVersion must be %q", APIVersionV1))
	}
	if s.Kind != KindServiceSLO {
		errs = append(errs, fmt.Sprintf("kind must be %q", KindServiceSLO))
	}
	if strings.TrimSpace(s.Metadata.Name) == "" {
		errs = append(errs, "metadata.name is required")
	}
	if strings.TrimSpace(s.Metadata.Service) == "" {
		errs = append(errs, "metadata.service is required")
	}
	if strings.TrimSpace(s.Metadata.Project) == "" {
		errs = append(errs, "metadata.project is required")
	}
	if strings.TrimSpace(s.Metadata.Runbook) != "" && !validURL(s.Metadata.Runbook) {
		errs = append(errs, "metadata.runbook must start with http:// or https://")
	}
	if len(s.SLOs) == 0 {
		errs = append(errs, "at least one SLO is required")
	}

	template, templateErr := TemplateForService(s.Metadata.Service)
	if templateErr != nil {
		errs = append(errs, templateErr.Error())
	}
	if alertErr := validateAlerting(s.Alerting, template); alertErr != "" {
		errs = append(errs, alertErr)
	}

	for i, slo := range s.SLOs {
		prefix := fmt.Sprintf("slos[%d]", i)
		if strings.TrimSpace(slo.Name) == "" {
			errs = append(errs, fmt.Sprintf("%s.name is required", prefix))
		}
		if slo.Objective <= 0 || slo.Objective >= 100 {
			errs = append(errs, fmt.Sprintf("%s.objective must be between 0 and 100", prefix))
		}
		if periodErr := validatePeriod(slo.Period, slo.Window); periodErr != "" {
			errs = append(errs, fmt.Sprintf("%s.period: %s", prefix, periodErr))
		}
		if !validWindow(slo.Window) {
			errs = append(errs, fmt.Sprintf("%s.window must look like 30d, 1h, or 15m", prefix))
		} else if windowErr := validateWindowBounds(slo.Window); windowErr != "" {
			errs = append(errs, fmt.Sprintf("%s.window: %s", prefix, windowErr))
		}
		if overrideErr := validateSLOAlerting(slo.Alerting); overrideErr != "" {
			errs = append(errs, fmt.Sprintf("%s.alerting: %s", prefix, overrideErr))
		}
		sliErrs := validateSLI(slo.SLI, template)
		for _, err := range sliErrs {
			errs = append(errs, fmt.Sprintf("%s.sli: %s", prefix, err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validWindow(window string) bool {
	if window == "" {
		return false
	}
	return windowRe.MatchString(window)
}

func validatePeriod(period, window string) string {
	period = strings.TrimSpace(period)
	switch period {
	case "":
		return ""
	case "rolling":
		if !validWindow(window) {
			return "rolling period requires a valid window"
		}
		return ""
	case "calendar":
		if !validCalendarWindow(window) {
			return "calendar period requires window of 1d, 1w, 2w, or 30d"
		}
		return ""
	default:
		return "must be rolling or calendar"
	}
}

func validateAlerting(alerting Alerting, template ServiceTemplate) string {
	if strings.TrimSpace(alerting.BurnRateResourceType) == "" {
		return ""
	}
	if template.ResourceType != "" && alerting.BurnRateResourceType != template.ResourceType {
		return fmt.Sprintf("alerting.burnRateResourceType must match template resource.type %q", template.ResourceType)
	}
	for _, r := range alerting.BurnRateResourceType {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return "burnRateResourceType must contain only lowercase letters, digits, or underscores"
	}
	return ""
}

func validCalendarWindow(window string) bool {
	switch window {
	case "1d", "1w", "2w", "30d":
		return true
	default:
		return false
	}
}

func validateSLI(sli SLI, template ServiceTemplate) []string {
	var errs []string
	switch sli.Type {
	case "request-based":
		if sli.Good == nil || sli.Total == nil {
			errs = append(errs, "request-based SLI requires good and total metrics")
			return errs
		}
		if strings.TrimSpace(sli.Good.Metric) == "" {
			errs = append(errs, "good.metric is required")
		}
		if strings.TrimSpace(sli.Total.Metric) == "" {
			errs = append(errs, "total.metric is required")
		}
		if strings.TrimSpace(sli.Good.Filter) == "" {
			errs = append(errs, "good.filter is required")
		} else if !qualifiedFilter(sli.Good.Filter) {
			errs = append(errs, "good.filter must reference metric., resource., project., metadata., or group.")
		}
		if strings.TrimSpace(sli.Total.Filter) != "" && !qualifiedFilter(sli.Total.Filter) {
			errs = append(errs, "total.filter must reference metric., resource., project., metadata., or group.")
		}
		if template.Name != "" {
			if err := template.ValidateMetric(sli.Good.Metric); err != nil {
				errs = append(errs, err.Error())
			}
			if err := template.ValidateMetric(sli.Total.Metric); err != nil {
				errs = append(errs, err.Error())
			}
			if !filterHasResource(sli.Good.Filter, template.ResourceType) {
				errs = append(errs, fmt.Sprintf("good.filter must include resource.type=%q", template.ResourceType))
			}
			if strings.TrimSpace(sli.Total.Filter) != "" && !filterHasResource(sli.Total.Filter, template.ResourceType) {
				errs = append(errs, fmt.Sprintf("total.filter must include resource.type=%q", template.ResourceType))
			}
		}
	case "latency":
		if strings.TrimSpace(sli.Metric) == "" {
			errs = append(errs, "metric is required")
		}
		if strings.TrimSpace(sli.Threshold) == "" {
			errs = append(errs, "threshold is required")
		} else {
			if _, err := time.ParseDuration(strings.TrimSpace(sli.Threshold)); err != nil {
				errs = append(errs, "threshold must be a valid duration like 500ms or 1s")
			}
		}
		if strings.TrimSpace(sli.Filter) != "" && !qualifiedFilter(sli.Filter) {
			errs = append(errs, "filter must reference metric., resource., project., metadata., or group.")
		}
		if template.Name != "" {
			if err := template.ValidateMetric(sli.Metric); err != nil {
				errs = append(errs, err.Error())
			}
			if !filterHasResource(sli.Filter, template.ResourceType) {
				errs = append(errs, fmt.Sprintf("filter must include resource.type=%q", template.ResourceType))
			}
		}
	default:
		errs = append(errs, "type must be request-based or latency")
	}
	return errs
}

func validateSLOAlerting(alerting SLOAlerting) string {
	var errs []string
	if err := validateAlertOverride("fast", alerting.Fast); err != "" {
		errs = append(errs, err)
	}
	if err := validateAlertOverride("slow", alerting.Slow); err != "" {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return strings.Join(errs, "; ")
	}
	return ""
}

func validateAlertOverride(name string, override *AlertOverride) string {
	if override == nil {
		return ""
	}
	var errs []string
	if len(override.Windows) > 0 {
		if len(override.Windows) != 2 {
			errs = append(errs, fmt.Sprintf("%s.windows must have exactly 2 entries", name))
		}
		if len(override.Windows) == 2 {
			d0, err0 := parseWindowDuration(override.Windows[0])
			d1, err1 := parseWindowDuration(override.Windows[1])
			if err0 != nil || err1 != nil {
				errs = append(errs, fmt.Sprintf("%s.windows must look like 30d, 1h, or 15m", name))
			} else {
				if d0 == d1 {
					errs = append(errs, fmt.Sprintf("%s.windows must have distinct short/long windows", name))
				}
				if d0 >= d1 {
					errs = append(errs, fmt.Sprintf("%s.windows must be ordered short, long", name))
				}
			}
		}
		for _, window := range override.Windows {
			if !validWindow(window) {
				errs = append(errs, fmt.Sprintf("%s.windows value %q must look like 30d, 1h, or 15m", name, window))
			}
		}
	}
	if override.BurnRate < 1 {
		errs = append(errs, fmt.Sprintf("%s.burnRate must be >= 1", name))
	}
	if len(errs) > 0 {
		return strings.Join(errs, "; ")
	}
	return ""
}

func filterHasResource(filter, resourceType string) bool {
	if strings.TrimSpace(resourceType) == "" {
		return true
	}
	return strings.Contains(filter, fmt.Sprintf("resource.type=\"%s\"", resourceType)) ||
		strings.Contains(filter, fmt.Sprintf("resource.type=%q", resourceType))
}

func validateWindowBounds(window string) string {
	d, err := parseWindowDuration(window)
	if err != nil {
		return err.Error()
	}
	if d < time.Minute {
		return "window must be at least 1m"
	}
	if d > 90*24*time.Hour {
		return "window must be 90d or less"
	}
	return ""
}

func parseWindowDuration(window string) (time.Duration, error) {
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

func validURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func qualifiedFilter(filter string) bool {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "metric.") ||
		strings.Contains(trimmed, "resource.") ||
		strings.Contains(trimmed, "project.") ||
		strings.Contains(trimmed, "metadata.") ||
		strings.Contains(trimmed, "group.")
}
