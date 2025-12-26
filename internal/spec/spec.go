package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	APIVersionV1   = "margin.dev/v1"
	KindServiceSLO = "ServiceSLO"
)

type Spec struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	SLOs       []SLO    `yaml:"slos"`
}

type Metadata struct {
	Name    string            `yaml:"name"`
	Service string            `yaml:"service"`
	Project string            `yaml:"project"`
	Labels  map[string]string `yaml:"labels"`
	Runbook string            `yaml:"runbook"`
}

type SLO struct {
	Name      string  `yaml:"name"`
	Objective float64 `yaml:"objective"`
	Window    string  `yaml:"window"`
	SLI       SLI     `yaml:"sli"`
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
	if len(s.SLOs) == 0 {
		errs = append(errs, "at least one SLO is required")
	}

	template, templateErr := TemplateForService(s.Metadata.Service)
	if templateErr != nil {
		errs = append(errs, templateErr.Error())
	}

	for i, slo := range s.SLOs {
		prefix := fmt.Sprintf("slos[%d]", i)
		if strings.TrimSpace(slo.Name) == "" {
			errs = append(errs, fmt.Sprintf("%s.name is required", prefix))
		}
		if slo.Objective <= 0 || slo.Objective >= 100 {
			errs = append(errs, fmt.Sprintf("%s.objective must be between 0 and 100", prefix))
		}
		if !validWindow(slo.Window) {
			errs = append(errs, fmt.Sprintf("%s.window must look like 30d, 1h, or 15m", prefix))
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
		}
		if template.Name != "" {
			if err := template.ValidateMetric(sli.Good.Metric); err != nil {
				errs = append(errs, err.Error())
			}
			if err := template.ValidateMetric(sli.Total.Metric); err != nil {
				errs = append(errs, err.Error())
			}
		}
	case "latency":
		if strings.TrimSpace(sli.Metric) == "" {
			errs = append(errs, "metric is required")
		}
		if strings.TrimSpace(sli.Threshold) == "" {
			errs = append(errs, "threshold is required")
		}
		if template.Name != "" {
			if err := template.ValidateMetric(sli.Metric); err != nil {
				errs = append(errs, err.Error())
			}
		}
	default:
		errs = append(errs, "type must be request-based or latency")
	}
	return errs
}
