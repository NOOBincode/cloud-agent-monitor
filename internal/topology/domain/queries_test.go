package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildQuery(t *testing.T) {
	t.Run("no args returns template as-is", func(t *testing.T) {
		result := BuildQuery("up{job=\"test\"}")
		assert.Equal(t, "up{job=\"test\"}", result)
	})

	t.Run("single placeholder", func(t *testing.T) {
		result := BuildQuery("up{job=\"%s\"}", "myapp")
		assert.Equal(t, "up{job=\"myapp\"}", result)
	})

	t.Run("multiple placeholders", func(t *testing.T) {
		result := BuildQuery("rate(http_requests_total{job=\"%s\",namespace=\"%s\"}[5m])", "myapp", "production")
		assert.Equal(t, "rate(http_requests_total{job=\"myapp\",namespace=\"production\"}[5m])", result)
	})

	t.Run("more placeholders than args reuses last arg", func(t *testing.T) {
		result := BuildQuery("rate(http_requests_total{job=\"%s\",namespace=\"%s\",code=\"%s\"}[5m])", "myapp", "production")
		assert.Contains(t, result, "job=\"myapp\"")
		assert.Contains(t, result, "namespace=\"production\"")
		assert.Contains(t, result, "code=\"production\"")
	})

	t.Run("no placeholders with args", func(t *testing.T) {
		result := BuildQuery("up", "unused")
		assert.Equal(t, "up", result)
	})
}

func TestBuildRequestRateQuery(t *testing.T) {
	result := BuildRequestRateQuery("myapp", "production")
	assert.Contains(t, result, "job=\"myapp\"")
	assert.Contains(t, result, "namespace=\"production\"")
	assert.Contains(t, result, "http_requests_total")
}

func TestBuildErrorRateQuery(t *testing.T) {
	result := BuildErrorRateQuery("myapp", "production")
	assert.Contains(t, result, "http_requests_total")
	assert.Contains(t, result, "code=~\"5..\"")
}

func TestBuildLatencyP99Query(t *testing.T) {
	result := BuildLatencyP99Query("myapp", "production")
	assert.Contains(t, result, "histogram_quantile(0.99")
	assert.Contains(t, result, "http_request_duration_seconds")
}

func TestBuildHTTPClientCallsQuery(t *testing.T) {
	result := BuildHTTPClientCallsQuery("myapp", "production")
	assert.Contains(t, result, "http_client_requests_total")
}

func TestBuildGRPCClientCallsQuery(t *testing.T) {
	result := BuildGRPCClientCallsQuery("myapp", "production")
	assert.Contains(t, result, "grpc_client_handled_total")
}

func TestServiceDiscoveryQueries(t *testing.T) {
	assert.Contains(t, ServiceDiscoveryQueries, "service_up")
	assert.Contains(t, ServiceDiscoveryQueries, "service_info")
	assert.Equal(t, "service_up", ServiceDiscoveryQueries["service_up"].Name)
}

func TestServiceMetricQueries(t *testing.T) {
	expectedKeys := []string{
		"request_rate", "request_rate_grpc",
		"error_rate", "error_rate_grpc",
		"latency_p99", "latency_p95", "latency_p50",
		"latency_p99_grpc",
	}
	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			q, ok := ServiceMetricQueries[key]
			assert.True(t, ok, "ServiceMetricQueries should contain key: %s", key)
			assert.NotEmpty(t, q.Name)
			assert.NotEmpty(t, q.Template)
		})
	}
}

func TestDependencyDiscoveryQueries(t *testing.T) {
	expectedKeys := []string{
		"http_client_calls", "grpc_client_calls",
		"database_connections", "cache_connections",
	}
	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			q, ok := DependencyDiscoveryQueries[key]
			assert.True(t, ok, "DependencyDiscoveryQueries should contain key: %s", key)
			assert.NotEmpty(t, q.Name)
		})
	}
}

func TestNetworkMetricQueries(t *testing.T) {
	expectedKeys := []string{
		"network_connections", "network_bytes_in",
		"network_bytes_out", "network_packet_loss",
	}
	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			q, ok := NetworkMetricQueries[key]
			assert.True(t, ok, "NetworkMetricQueries should contain key: %s", key)
			assert.NotEmpty(t, q.Name)
		})
	}
}

func TestTopologyQuery_GetDepth(t *testing.T) {
	tests := []struct {
		name  string
		query TopologyQuery
		want  int
	}{
		{
			name:  "with depth",
			query: TopologyQuery{Depth: 5},
			want:  5,
		},
		{
			name:  "zero depth uses default",
			query: TopologyQuery{Depth: 0},
			want:  3,
		},
		{
			name:  "depth exceeds max",
			query: TopologyQuery{Depth: 15},
			want:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.query.GetDepth())
		})
	}
}

func TestTopologyQuery_GetLimit(t *testing.T) {
	tests := []struct {
		name  string
		query TopologyQuery
		want  int
	}{
		{
			name:  "with limit",
			query: TopologyQuery{Limit: 50},
			want:  50,
		},
		{
			name:  "zero limit uses default",
			query: TopologyQuery{Limit: 0},
			want:  100,
		},
		{
			name:  "limit exceeds max",
			query: TopologyQuery{Limit: 20000},
			want:  10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.query.GetLimit())
		})
	}
}

func TestTopologyQuery_HasTimeRange(t *testing.T) {
	startTime := mustParseTime("2024-01-01T00:00:00Z")
	endTime := mustParseTime("2024-01-02T00:00:00Z")

	tests := []struct {
		name  string
		query TopologyQuery
		want  bool
	}{
		{
			name:  "with start and end time",
			query: TopologyQuery{StartTime: &startTime, EndTime: &endTime},
			want:  true,
		},
		{
			name:  "only start time",
			query: TopologyQuery{StartTime: &startTime},
			want:  false,
		},
		{
			name:  "no time range",
			query: TopologyQuery{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.query.HasTimeRange())
		})
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
