package parser

import (
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

// PanelQuery represents a parsed query from a dashboard panel
type PanelQuery struct {
	PanelTitle string
	QueryExpr  string
	Datasource string
	Component  string
	MetricName string
}

// Dashboard represents a minimal dashboard structure for parsing
type Dashboard struct {
	Panels []Panel
}

// Panel represents a dashboard panel
type Panel struct {
	ID      int
	Title   string
	Type    string
	Targets []Target
}

// Target represents a query target in a panel
type Target struct {
	Expr       string
	Datasource *Datasource
}

// Datasource represents a datasource reference
type Datasource struct {
	Type string
	UID  string
}

// ParseDashboardFromMap extracts queries from a dashboard map (from Grafana OpenAPI)
func ParseDashboardFromMap(dashboardMap map[string]interface{}) []PanelQuery {
	dashboard := convertMapToDashboard(dashboardMap)
	return ParseDashboard(&dashboard)
}

// convertMapToDashboard converts a dashboard map to our Dashboard struct
func convertMapToDashboard(dashboardMap map[string]interface{}) Dashboard {
	dashboard := Dashboard{}

	if panelsRaw, ok := dashboardMap["panels"].([]interface{}); ok {
		for _, panelRaw := range panelsRaw {
			if panelMap, ok := panelRaw.(map[string]interface{}); ok {
				panel := Panel{}

				if id, ok := panelMap["id"].(float64); ok {
					panel.ID = int(id)
				}
				if title, ok := panelMap["title"].(string); ok {
					panel.Title = title
				}
				if ptype, ok := panelMap["type"].(string); ok {
					panel.Type = ptype
				}

				if targetsRaw, ok := panelMap["targets"].([]interface{}); ok {
					for _, targetRaw := range targetsRaw {
						if targetMap, ok := targetRaw.(map[string]interface{}); ok {
							target := Target{}

							if expr, ok := targetMap["expr"].(string); ok {
								target.Expr = expr
							}

							if dsRaw, ok := targetMap["datasource"].(map[string]interface{}); ok {
								ds := &Datasource{}
								if dsType, ok := dsRaw["type"].(string); ok {
									ds.Type = dsType
								}
								if dsUID, ok := dsRaw["uid"].(string); ok {
									ds.UID = dsUID
								}
								target.Datasource = ds
							}

							panel.Targets = append(panel.Targets, target)
						}
					}
				}

				dashboard.Panels = append(dashboard.Panels, panel)
			}
		}
	}

	return dashboard
}

// substituteGrafanaVariables replaces Grafana template variables with PromQL patterns
// and removes cluster/job label matchers since we don't care about those
func substituteGrafanaVariables(expr string) string {
	// First, replace Grafana-specific variables that aren't valid PromQL
	result := expr

	// Replace namespace variables
	result = strings.ReplaceAll(result, `namespace="$namespace"`, `namespace=~".*"`)
	result = strings.ReplaceAll(result, `namespace="${namespace}"`, `namespace=~".*"`)
	result = strings.ReplaceAll(result, `namespace=~"$namespace"`, `namespace=~".*"`)
	result = strings.ReplaceAll(result, `namespace=~"${namespace}"`, `namespace=~".*"`)

	// Replace Grafana interval/range variables
	result = strings.ReplaceAll(result, "$__rate_interval", "5m")
	result = strings.ReplaceAll(result, "${__rate_interval}", "5m")
	result = strings.ReplaceAll(result, "$__interval", "5m")
	result = strings.ReplaceAll(result, "${__interval}", "5m")
	result = strings.ReplaceAll(result, "$__range", "3600")
	result = strings.ReplaceAll(result, "${__range}", "3600")
	result = strings.ReplaceAll(result, "$__range_s", "3600")
	result = strings.ReplaceAll(result, "${__range_s}", "3600")
	result = strings.ReplaceAll(result, "$__range_ms", "3600000")
	result = strings.ReplaceAll(result, "${__range_ms}", "3600000")

	// Now parse with PromQL parser and remove label matchers with $ variables
	expr2, err := promParser.ParseExpr(result)
	if err != nil {
		// If parse fails, return original with basic cleanup
		return result
	}

	// Walk the AST and remove label matchers containing $, cluster, or job with wildcards
	promParser.Inspect(expr2, func(node promParser.Node, path []promParser.Node) error {
		if n, ok := node.(*promParser.VectorSelector); ok {
			// Filter label matchers
			newMatchers := make([]*labels.Matcher, 0)
			for _, m := range n.LabelMatchers {
				// Skip if it's the __name__ matcher
				if m.Name == "__name__" {
					newMatchers = append(newMatchers, m)
					continue
				}

				// Remove matchers with $ variables
				if strings.Contains(m.Value, "$") {
					continue
				}

				// Remove cluster matchers with wildcards
				if m.Name == "cluster" && strings.Contains(m.Value, "*") {
					continue
				}

				// Remove job matchers with wildcards or $
				if m.Name == "job" && (strings.Contains(m.Value, "*") || strings.Contains(m.Value, "$")) {
					continue
				}

				// Keep this matcher
				newMatchers = append(newMatchers, m)
			}
			n.LabelMatchers = newMatchers
		}
		return nil
	})

	// Convert back to string
	return expr2.String()
}

// ParseDashboard extracts queries from dashboard panels
func ParseDashboard(dashboard *Dashboard) []PanelQuery {
	var queries []PanelQuery

	for _, panel := range dashboard.Panels {
		// Skip row panels
		if panel.Type == "row" {
			continue
		}

		for _, target := range panel.Targets {
			if target.Expr == "" {
				continue
			}

			// Skip non-Prometheus datasources
			datasourceType := "prometheus"
			if target.Datasource != nil {
				datasourceType = target.Datasource.Type
			}
			if datasourceType != "prometheus" {
				continue
			}

			// Clean up the query expression
			cleanExpr := substituteGrafanaVariables(target.Expr)

			// Extract component name from the query
			component := extractComponentFromQuery(cleanExpr)

			// Extract metric name
			metricName := extractMetricNameFromQuery(cleanExpr)

			queries = append(queries, PanelQuery{
				PanelTitle: panel.Title,
				QueryExpr:  cleanExpr,
				Datasource: datasourceType,
				Component:  component,
				MetricName: metricName,
			})
		}
	}

	return queries
}

// extractComponentFromQuery attempts to extract the component/service name from a query
func extractComponentFromQuery(expr string) string {
	// Try to parse the PromQL expression
	parsedExpr, err := promParser.ParseExpr(expr)
	if err != nil {
		return "unknown"
	}

	component := "unknown"

	// Walk the AST to find label matchers
	promParser.Inspect(parsedExpr, func(node promParser.Node, path []promParser.Node) error {
		if n, ok := node.(*promParser.VectorSelector); ok {
			for _, m := range n.LabelMatchers {
				// Look for common component label names
				if m.Name == "component" || m.Name == "job" || m.Name == "container" {
					// Extract the component name, handling regex patterns
					val := m.Value
					val = strings.Trim(val, "\"")
					val = strings.ReplaceAll(val, ".*", "")
					val = strings.ReplaceAll(val, ".+", "")
					val = strings.ReplaceAll(val, "^", "")
					val = strings.ReplaceAll(val, "$", "")
					if val != "" && !strings.Contains(val, "*") {
						component = val
						return nil
					}
				}
			}
		}
		return nil
	})

	return component
}

// extractMetricNameFromQuery extracts the metric name from a query
func extractMetricNameFromQuery(expr string) string {
	// Try to parse the PromQL expression
	parsedExpr, err := promParser.ParseExpr(expr)
	if err != nil {
		// Fallback: try simple regex extraction
		re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)`)
		matches := re.FindStringSubmatch(expr)
		if len(matches) > 1 {
			return matches[1]
		}
		return "unknown"
	}

	metricName := "unknown"

	// Walk the AST to find the metric name
	promParser.Inspect(parsedExpr, func(node promParser.Node, path []promParser.Node) error {
		if n, ok := node.(*promParser.VectorSelector); ok {
			// Look for the __name__ matcher
			for _, m := range n.LabelMatchers {
				if m.Name == "__name__" {
					metricName = m.Value
					return nil
				}
			}
			// If no __name__ matcher, use the metric name from the selector
			if n.Name != "" {
				metricName = n.Name
				return nil
			}
		}
		return nil
	})

	return metricName
}

// GroupQueriesByComponent groups queries by component name
func GroupQueriesByComponent(queries []PanelQuery) map[string][]PanelQuery {
	grouped := make(map[string][]PanelQuery)
	for _, q := range queries {
		grouped[q.Component] = append(grouped[q.Component], q)
	}
	return grouped
}

// ExtractComponents returns a list of unique component names from queries
func ExtractComponents(queries []PanelQuery) []string {
	componentMap := make(map[string]bool)
	for _, q := range queries {
		if q.Component != "unknown" && q.Component != "" {
			componentMap[q.Component] = true
		}
	}

	components := make([]string, 0, len(componentMap))
	for comp := range componentMap {
		components = append(components, comp)
	}
	return components
}
