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
	"gke-ingress": {
		Name:         "gke-ingress",
		ResourceType: "k8s_ingress",
		Metrics: map[string]MetricTemplate{
			"kubernetes.io/ingress/request_count": {
				Name:        "kubernetes.io/ingress/request_count",
				Description: "GKE ingress request count",
			},
			"kubernetes.io/ingress/latency": {
				Name:        "kubernetes.io/ingress/latency",
				Description: "GKE ingress request latency distribution",
			},
		},
		Pitfalls: []string{
			"Default backend 404s can mask real availability issues.",
			"Ingress metrics are per-cluster; multi-cluster routing may need multiple SLOs.",
		},
	},
	"cloud-sql": {
		Name:         "cloud-sql",
		ResourceType: "cloudsql_database",
		Metrics: map[string]MetricTemplate{
			"cloudsql.googleapis.com/database/queries": {
				Name:        "cloudsql.googleapis.com/database/queries",
				Description: "Cloud SQL query count",
			},
			"cloudsql.googleapis.com/database/query_latency": {
				Name:        "cloudsql.googleapis.com/database/query_latency",
				Description: "Cloud SQL query latency distribution",
			},
		},
		Pitfalls: []string{
			"Long-running queries can skew latency SLOs without proper filters.",
			"Replica failover can create transient errors that impact availability.",
		},
	},
	"gke-service": {
		Name:         "gke-service",
		ResourceType: "k8s_service",
		Metrics: map[string]MetricTemplate{
			"kubernetes.io/service/request_count": {
				Name:        "kubernetes.io/service/request_count",
				Description: "GKE service request count",
			},
			"kubernetes.io/service/latency": {
				Name:        "kubernetes.io/service/latency",
				Description: "GKE service request latency distribution",
			},
		},
		Pitfalls: []string{
			"Service metrics are per-cluster; multi-cluster services need multiple SLOs.",
			"Mixing readiness probe failures with user traffic can skew availability.",
		},
	},
	"gke-gateway": {
		Name:         "gke-gateway",
		ResourceType: "k8s_gateway",
		Metrics: map[string]MetricTemplate{
			"kubernetes.io/gateway/request_count": {
				Name:        "kubernetes.io/gateway/request_count",
				Description: "GKE Gateway request count",
			},
			"kubernetes.io/gateway/latency": {
				Name:        "kubernetes.io/gateway/latency",
				Description: "GKE Gateway request latency distribution",
			},
		},
		Pitfalls: []string{
			"Gateway metrics can include internal health checks unless filtered.",
			"Gateway routing rules may mask backend-specific latency issues.",
		},
	},
	"gce-lb": {
		Name:         "gce-lb",
		ResourceType: "gce_forwarding_rule",
		Metrics: map[string]MetricTemplate{
			"loadbalancing.googleapis.com/https/request_count": {
				Name:        "loadbalancing.googleapis.com/https/request_count",
				Description: "HTTPS load balancer request count (GCE)",
			},
			"loadbalancing.googleapis.com/https/total_latencies": {
				Name:        "loadbalancing.googleapis.com/https/total_latencies",
				Description: "HTTPS load balancer latency distribution (GCE)",
			},
		},
		Pitfalls: []string{
			"Backend errors can be masked by cache hits without proper filters.",
			"Global vs regional load balancers may use different resource labels.",
		},
	},
	"cloud-functions": {
		Name:         "cloud-functions",
		ResourceType: "cloud_function",
		Metrics: map[string]MetricTemplate{
			"cloudfunctions.googleapis.com/function/execution_count": {
				Name:        "cloudfunctions.googleapis.com/function/execution_count",
				Description: "Cloud Functions execution count",
			},
			"cloudfunctions.googleapis.com/function/execution_times": {
				Name:        "cloudfunctions.googleapis.com/function/execution_times",
				Description: "Cloud Functions execution time distribution",
			},
		},
		Pitfalls: []string{
			"Cold starts can inflate latency for low-traffic functions.",
			"Retries can double-count failures unless filtered.",
		},
	},
	"pubsub-subscription": {
		Name:         "pubsub-subscription",
		ResourceType: "pubsub_subscription",
		Metrics: map[string]MetricTemplate{
			"pubsub.googleapis.com/subscription/ack_message_count": {
				Name:        "pubsub.googleapis.com/subscription/ack_message_count",
				Description: "Pub/Sub acked message count",
			},
			"pubsub.googleapis.com/subscription/ack_message_delay": {
				Name:        "pubsub.googleapis.com/subscription/ack_message_delay",
				Description: "Pub/Sub ack delay distribution",
			},
		},
		Pitfalls: []string{
			"Backlog spikes can be caused by subscriber scaling, not publisher errors.",
			"Dead-letter policies can hide underlying delivery failures.",
		},
	},
	"cloud-storage": {
		Name:         "cloud-storage",
		ResourceType: "gcs_bucket",
		Metrics: map[string]MetricTemplate{
			"storage.googleapis.com/api/request_count": {
				Name:        "storage.googleapis.com/api/request_count",
				Description: "Cloud Storage API request count",
			},
			"storage.googleapis.com/api/request_latencies": {
				Name:        "storage.googleapis.com/api/request_latencies",
				Description: "Cloud Storage API request latency distribution",
			},
		},
		Pitfalls: []string{
			"Multi-region buckets can have higher tail latency without an incident.",
			"Requester-pays or IAM errors can look like availability issues.",
		},
	},
	"cloud-tasks": {
		Name:         "cloud-tasks",
		ResourceType: "cloud_tasks_queue",
		Metrics: map[string]MetricTemplate{
			"cloudtasks.googleapis.com/queue/task_attempt_count": {
				Name:        "cloudtasks.googleapis.com/queue/task_attempt_count",
				Description: "Cloud Tasks task attempt count",
			},
			"cloudtasks.googleapis.com/queue/task_attempt_latencies": {
				Name:        "cloudtasks.googleapis.com/queue/task_attempt_latencies",
				Description: "Cloud Tasks task attempt latency distribution",
			},
		},
		Pitfalls: []string{
			"High retry rates can inflate attempts without real user impact.",
			"Queue throttling may increase latency during bursts.",
		},
	},
	"bigquery": {
		Name:         "bigquery",
		ResourceType: "bigquery_project",
		Metrics: map[string]MetricTemplate{
			"bigquery.googleapis.com/query/count": {
				Name:        "bigquery.googleapis.com/query/count",
				Description: "BigQuery query count",
			},
			"bigquery.googleapis.com/query/latency": {
				Name:        "bigquery.googleapis.com/query/latency",
				Description: "BigQuery query latency distribution",
			},
		},
		Pitfalls: []string{
			"Batch queries have higher latency and should be filtered separately.",
			"Resource-heavy queries can dominate latency even when the service is healthy.",
		},
	},
	"spanner": {
		Name:         "spanner",
		ResourceType: "spanner_instance",
		Metrics: map[string]MetricTemplate{
			"spanner.googleapis.com/api/request_count": {
				Name:        "spanner.googleapis.com/api/request_count",
				Description: "Spanner API request count",
			},
			"spanner.googleapis.com/api/latency": {
				Name:        "spanner.googleapis.com/api/latency",
				Description: "Spanner API latency distribution",
			},
		},
		Pitfalls: []string{
			"Hot partitions can cause latency spikes without full outage.",
			"Client-side timeouts can appear as service errors unless filtered.",
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
