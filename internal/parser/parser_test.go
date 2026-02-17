package parser

import (
	"testing"
)

func TestParseDashboardFromMap(t *testing.T) {
	// Create a sample dashboard map
	dashboardMap := map[string]interface{}{
		"title": "Test Dashboard",
		"panels": []interface{}{
			map[string]interface{}{
				"id":    float64(1),
				"title": "CPU Usage",
				"type":  "graph",
				"targets": []interface{}{
					map[string]interface{}{
						"expr": `rate(container_cpu_usage_seconds_total{namespace="tempo",container="distributor"}[5m])`,
						"datasource": map[string]interface{}{
							"type": "prometheus",
							"uid":  "prom-uid",
						},
					},
				},
			},
			map[string]interface{}{
				"id":    float64(2),
				"title": "Memory Usage",
				"type":  "graph",
				"targets": []interface{}{
					map[string]interface{}{
						"expr": `container_memory_working_set_bytes{namespace="tempo",container="ingester"}`,
						"datasource": map[string]interface{}{
							"type": "prometheus",
							"uid":  "prom-uid",
						},
					},
				},
			},
			// Row panel should be skipped
			map[string]interface{}{
				"id":    float64(3),
				"title": "Row Title",
				"type":  "row",
			},
		},
	}

	queries := ParseDashboardFromMap(dashboardMap)

	if len(queries) != 2 {
		t.Errorf("Expected 2 queries, got %d", len(queries))
	}

	// Check first query
	if queries[0].PanelTitle != "CPU Usage" {
		t.Errorf("Expected panel title 'CPU Usage', got '%s'", queries[0].PanelTitle)
	}

	if queries[0].Component == "" || queries[0].Component == "unknown" {
		t.Errorf("Expected component to be detected, got '%s'", queries[0].Component)
	}

	// Check second query
	if queries[1].PanelTitle != "Memory Usage" {
		t.Errorf("Expected panel title 'Memory Usage', got '%s'", queries[1].PanelTitle)
	}
}

func TestExtractComponents(t *testing.T) {
	queries := []PanelQuery{
		{Component: "distributor", MetricName: "cpu"},
		{Component: "distributor", MetricName: "memory"},
		{Component: "ingester", MetricName: "cpu"},
		{Component: "unknown", MetricName: "test"},
		{Component: "querier", MetricName: "requests"},
	}

	components := ExtractComponents(queries)

	expectedCount := 3 // distributor, ingester, querier (unknown excluded)
	if len(components) != expectedCount {
		t.Errorf("Expected %d components, got %d: %v", expectedCount, len(components), components)
	}

	// Check that unknown is not included
	for _, comp := range components {
		if comp == "unknown" {
			t.Error("'unknown' component should not be included in results")
		}
	}
}

func TestGroupQueriesByComponent(t *testing.T) {
	queries := []PanelQuery{
		{Component: "distributor", MetricName: "cpu", PanelTitle: "CPU 1"},
		{Component: "distributor", MetricName: "memory", PanelTitle: "Memory 1"},
		{Component: "ingester", MetricName: "cpu", PanelTitle: "CPU 2"},
	}

	grouped := GroupQueriesByComponent(queries)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 component groups, got %d", len(grouped))
	}

	if len(grouped["distributor"]) != 2 {
		t.Errorf("Expected 2 queries for distributor, got %d", len(grouped["distributor"]))
	}

	if len(grouped["ingester"]) != 1 {
		t.Errorf("Expected 1 query for ingester, got %d", len(grouped["ingester"]))
	}
}

func TestExtractComponentFromQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		expected  string
	}{
		{
			name:     "component label",
			query:    `up{component="distributor"}`,
			expected: "distributor",
		},
		{
			name:     "container label",
			query:    `rate(cpu_usage{container="ingester"}[5m])`,
			expected: "ingester",
		},
		{
			name:     "job label with wildcard",
			query:    `rate(requests{job=~".*querier.*"}[5m])`,
			expected: "querier",
		},
		{
			name:     "no component",
			query:    `up{namespace="default"}`,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractComponentFromQuery(tt.query)
			if result != tt.expected {
				t.Errorf("extractComponentFromQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractMetricNameFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple metric",
			query:    `up`,
			expected: "up",
		},
		{
			name:     "metric with labels",
			query:    `cpu_usage{container="test"}`,
			expected: "cpu_usage",
		},
		{
			name:     "metric in function",
			query:    `rate(http_requests_total[5m])`,
			expected: "http_requests_total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMetricNameFromQuery(tt.query)
			if result != tt.expected {
				t.Errorf("extractMetricNameFromQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}
