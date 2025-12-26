package spec

import (
	"fmt"
	"sort"
)

type ServiceTemplate struct {
	Name         string
	ResourceType string
	Metrics      map[string]MetricTemplate
	Pitfalls     []string
}

type MetricTemplate struct {
	Name        string
	Description string
}

var serviceTemplates = map[string]ServiceTemplate{
	"cloud-run": {
		Name:         "cloud-run",
		ResourceType: "cloud_run_revision",
		Metrics: map[string]MetricTemplate{
			"run.googleapis.com/request_count": {
				Name:        "run.googleapis.com/request_count",
				Description: "Request count for Cloud Run services",
			},
			"run.googleapis.com/request_latencies": {
				Name:        "run.googleapis.com/request_latencies",
				Description: "Request latency distribution for Cloud Run",
			},
		},
		Pitfalls: []string{
			"Cold starts can skew latency SLOs for low-traffic services.",
			"Retries can double-count failed requests unless filters exclude them.",
		},
	},
	"https-load-balancer": {
		Name:         "https-load-balancer",
		ResourceType: "https_lb_rule",
		Metrics: map[string]MetricTemplate{
			"loadbalancing.googleapis.com/https/request_count": {
				Name:        "loadbalancing.googleapis.com/https/request_count",
				Description: "HTTPS load balancer request count",
			},
			"loadbalancing.googleapis.com/https/total_latencies": {
				Name:        "loadbalancing.googleapis.com/https/total_latencies",
				Description: "HTTPS load balancer total latency distribution",
			},
		},
		Pitfalls: []string{
			"Backends returning 404s can hide real availability issues.",
			"Retry policies may inflate request counts.",
		},
	},
}

func TemplateForService(service string) (ServiceTemplate, error) {
	if tpl, ok := serviceTemplates[service]; ok {
		return tpl, nil
	}
	var keys []string
	for k := range serviceTemplates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return ServiceTemplate{}, fmt.Errorf("metadata.service must be one of %v", keys)
}

func (t ServiceTemplate) ValidateMetric(metric string) error {
	if metric == "" {
		return fmt.Errorf("metric must not be empty")
	}
	if _, ok := t.Metrics[metric]; !ok {
		return fmt.Errorf("metric %q is not supported for service %q", metric, t.Name)
	}
	return nil
}
