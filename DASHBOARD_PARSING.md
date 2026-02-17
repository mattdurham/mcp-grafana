# Dashboard Parsing Features

This document describes the dashboard parsing and component analysis features added to mcp-grafana.

## Overview

These features allow you to parse Grafana dashboards to extract Prometheus queries, detect component/service names, and analyze dashboard metrics patterns. This is particularly useful for:

- **Service Discovery**: Automatically identify services/components monitored in dashboards
- **Query Analysis**: Extract and analyze Prometheus queries from dashboard panels
- **Documentation**: Generate documentation of what metrics each dashboard monitors
- **Migration**: Understand dashboard dependencies when migrating or refactoring

## New Tools

### 1. `parse_dashboard_queries`

Parses a dashboard to extract all Prometheus queries with detected component/service names.

**Parameters:**
- `uid` (required): The UID of the dashboard to parse

**Returns:**
```json
{
  "queries": [
    {
      "panel_title": "CPU Usage",
      "query": "rate(container_cpu_usage_seconds_total{namespace=~\".*\",container=\"distributor\"}[5m])",
      "component": "distributor",
      "metric_name": "container_cpu_usage_seconds_total"
    }
  ]
}
```

**Example Usage:**
```
Parse the queries from dashboard UID "abc123"
```

### 2. `list_dashboard_components`

Extracts a list of unique components/services from a dashboard's Prometheus queries.

**Parameters:**
- `uid` (required): The UID of the dashboard to analyze

**Returns:**
```json
{
  "components": [
    "distributor",
    "ingester",
    "querier",
    "compactor"
  ]
}
```

**Example Usage:**
```
What components/services are monitored in dashboard UID "abc123"?
```

## Parser Logic

The parser automatically:

1. **Extracts Queries**: Finds all Prometheus queries from dashboard panels
2. **Cleans Variables**: Substitutes Grafana template variables (like `$namespace`, `$__interval`) with patterns
3. **Detects Components**: Analyzes label matchers to identify component/service names from:
   - `component` labels
   - `job` labels
   - `container` labels
4. **Extracts Metrics**: Identifies the primary metric name from each query
5. **Groups Results**: Organizes queries by component for easy analysis

## Use Cases

### Service Inventory

Quickly discover what services are being monitored across your dashboards:

```
List all components in the "Tempo Operational" dashboard
```

### Query Documentation

Extract all queries for documentation or review:

```
Parse all queries from dashboard "tempo-ops" and show me the queries for the distributor component
```

### Dashboard Migration

When migrating dashboards, understand what queries and components they monitor:

```
What components does dashboard X monitor? I need to create a similar dashboard for a new environment.
```

### Metric Analysis

Analyze what metrics are being used:

```
Parse dashboard Y and tell me what metrics are used most frequently
```

## Implementation Details

### Component Detection

The parser looks for component names in common label matchers:
- `component="distributor"` → component: distributor
- `job=~".*ingester.*"` → component: ingester
- `container="querier"` → component: querier

### Variable Substitution

Grafana variables are substituted for analysis:
- `$namespace` → `namespace=~".*"`
- `$__rate_interval` → `5m`
- `$__interval` → `5m`

### Prometheus Parsing

Uses the official Prometheus PromQL parser to ensure accurate query parsing and component extraction.

## Integration

These tools integrate seamlessly with existing mcp-grafana tools:

1. Use `search_dashboards` to find dashboards
2. Use `parse_dashboard_queries` to analyze the dashboard
3. Use `get_dashboard_property` to extract specific panel configurations
4. Use Prometheus tools to test the extracted queries

## Examples

### Example 1: Service Discovery

```
User: What services are monitored in the Tempo dashboard?
Assistant: Let me search for the Tempo dashboard and analyze it.

1. search_dashboards(query="Tempo")
2. parse_dashboard_queries(uid="found-uid")
3. list_dashboard_components(uid="found-uid")

Result: The Tempo dashboard monitors: distributor, ingester, querier, compactor, metrics-generator
```

### Example 2: Query Analysis

```
User: Show me all the queries used for monitoring the distributor component
Assistant: I'll parse the dashboard and filter queries by component.

1. parse_dashboard_queries(uid="tempo-dashboard")
2. Filter results where component="distributor"

Result: Found 15 queries monitoring the distributor including CPU, memory, request rates, etc.
```

## Future Enhancements

Potential future additions:
- Multi-dashboard analysis across folders
- Query pattern detection and recommendations
- Alerting rule generation from dashboard queries
- Dashboard diff comparison
- Automatic component metrics catalog generation
