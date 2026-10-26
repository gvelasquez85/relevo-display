package yamchecks

import (
	"relevo/internal/session"
	"sort"
	"strings"
)

// SchemaSummary includes only approved event/field names and fixed type labels.
func SchemaSummary(schemas map[string]map[string]string) string {
	names := make([]string, 0, len(schemas))
	for n := range schemas {
		if _, ok := session.InputEventSchema(map[string]interface{}{"eventName": n}); ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	parts := []string{}
	fields := []string{"eventName", "x", "y", "width", "height", "mouseButton", "modifierFlags", "deltaX", "deltaY", "phase"}
	for _, n := range names {
		f := []string{}
		for _, k := range fields {
			v := schemas[n][k]
			switch v {
			case "string", "number", "boolean", "null", "object", "array", "other":
				f = append(f, k+":"+v)
			}
		}
		parts = append(parts, n+"{"+strings.Join(f, ",")+"}")
	}
	if len(parts) == 0 {
		return "ninguno"
	}
	return strings.Join(parts, " · ")
}
