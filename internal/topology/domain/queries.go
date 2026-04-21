package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	DefaultQueryRange = 5 * time.Minute
)

type QueryTemplate struct {
	Name        string
	Description string
	Template    string
}

var ServiceDiscoveryQueries = map[string]QueryTemplate{
	"service_up": {
		Name:        "service_up",
		Description: "服务存活状态",
		Template:    `up{job="%s"}`,
	},
	"service_info": {
		Name:        "service_info",
		Description: "服务信息（从服务发现获取）",
		Template:    `up{job="%s"}`,
	},
}

var ServiceMetricQueries = map[string]QueryTemplate{
	"request_rate": {
		Name:        "request_rate",
		Description: "服务请求率 (QPS)",
		Template:    `sum(rate(http_requests_total{job="%s",namespace="%s"}[5m])) by (service,namespace)`,
	},
	"request_rate_grpc": {
		Name:        "request_rate_grpc",
		Description: "gRPC 服务请求率",
		Template:    `sum(rate(grpc_server_handled_total{job="%s",namespace="%s"}[5m])) by (grpc_service,namespace)`,
	},
	"error_rate": {
		Name:        "error_rate",
		Description: "服务错误率",
		Template:    `sum(rate(http_requests_total{job="%s",namespace="%s",code=~"5.."}[5m])) by (service,namespace) / sum(rate(http_requests_total{job="%s",namespace="%s"}[5m])) by (service,namespace) * 100`,
	},
	"error_rate_grpc": {
		Name:        "error_rate_grpc",
		Description: "gRPC 服务错误率",
		Template:    `sum(rate(grpc_server_handled_total{job="%s",namespace="%s",grpc_code!="OK"}[5m])) by (grpc_service,namespace) / sum(rate(grpc_server_handled_total{job="%s",namespace="%s"}[5m])) by (grpc_service,namespace) * 100`,
	},
	"latency_p99": {
		Name:        "latency_p99",
		Description: "P99 延迟",
		Template:    `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="%s",namespace="%s"}[5m])) by (le,service,namespace))`,
	},
	"latency_p95": {
		Name:        "latency_p95",
		Description: "P95 延迟",
		Template:    `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{job="%s",namespace="%s"}[5m])) by (le,service,namespace))`,
	},
	"latency_p50": {
		Name:        "latency_p50",
		Description: "P50 延迟",
		Template:    `histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket{job="%s",namespace="%s"}[5m])) by (le,service,namespace))`,
	},
	"latency_p99_grpc": {
		Name:        "latency_p99_grpc",
		Description: "gRPC P99 延迟",
		Template:    `histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{job="%s",namespace="%s"}[5m])) by (le,grpc_service,namespace))`,
	},
}

var DependencyDiscoveryQueries = map[string]QueryTemplate{
	"http_client_calls": {
		Name:        "http_client_calls",
		Description: "HTTP 客户端调用关系",
		Template:    `sum(rate(http_client_requests_total{job="%s",namespace="%s"}[5m])) by (service,namespace,host)`,
	},
	"grpc_client_calls": {
		Name:        "grpc_client_calls",
		Description: "gRPC 客户端调用关系",
		Template:    `sum(rate(grpc_client_handled_total{job="%s",namespace="%s"}[5m])) by (service,namespace,grpc_service)`,
	},
	"database_connections": {
		Name:        "database_connections",
		Description: "数据库连接",
		Template:    `sum(db_client_connections{job="%s",namespace="%s"}) by (service,namespace,database)`,
	},
	"cache_connections": {
		Name:        "cache_connections",
		Description: "缓存连接",
		Template:    `sum(redis_connections{job="%s",namespace="%s"}) by (service,namespace)`,
	},
}

var NetworkMetricQueries = map[string]QueryTemplate{
	"network_connections": {
		Name:        "network_connections",
		Description: "网络连接数",
		Template:    `sum(net_conntrack_entries{job="%s"}) by (namespace,pod)`,
	},
	"network_bytes_in": {
		Name:        "network_bytes_in",
		Description: "入站流量",
		Template:    `sum(rate(container_network_receive_bytes_total{namespace="%s"}[5m])) by (namespace,pod)`,
	},
	"network_bytes_out": {
		Name:        "network_bytes_out",
		Description: "出站流量",
		Template:    `sum(rate(container_network_transmit_bytes_total{namespace="%s"}[5m])) by (namespace,pod)`,
	},
	"network_packet_loss": {
		Name:        "network_packet_loss",
		Description: "丢包率",
		Template:    `sum(rate(container_network_receive_packets_dropped_total{namespace="%s"}[5m])) by (namespace,pod) / sum(rate(container_network_receive_packets_total{namespace="%s"}[5m])) by (namespace,pod) * 100`,
	},
}

func BuildQuery(template string, args ...string) string {
	if len(args) == 0 {
		return template
	}

	placeholders := strings.Count(template, "%s")
	if placeholders == 0 {
		return template
	}

	expandedArgs := make([]interface{}, placeholders)
	for i := 0; i < placeholders; i++ {
		if i < len(args) {
			expandedArgs[i] = args[i]
		} else {
			expandedArgs[i] = args[len(args)-1]
		}
	}

	return fmt.Sprintf(template, expandedArgs...)
}

func BuildRequestRateQuery(job, namespace string) string {
	return BuildQuery(ServiceMetricQueries["request_rate"].Template, job, namespace)
}

func BuildErrorRateQuery(job, namespace string) string {
	return BuildQuery(ServiceMetricQueries["error_rate"].Template, job, namespace, job, namespace)
}

func BuildLatencyP99Query(job, namespace string) string {
	return BuildQuery(ServiceMetricQueries["latency_p99"].Template, job, namespace)
}

func BuildHTTPClientCallsQuery(job, namespace string) string {
	return BuildQuery(DependencyDiscoveryQueries["http_client_calls"].Template, job, namespace)
}

func BuildGRPCClientCallsQuery(job, namespace string) string {
	return BuildQuery(DependencyDiscoveryQueries["grpc_client_calls"].Template, job, namespace)
}
