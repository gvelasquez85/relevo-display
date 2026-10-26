package yamchecks

import (
	"strings"
	"testing"
)

func TestSchemaSummaryDropsRawOrUnknownFields(t *testing.T) {
	s := SchemaSummary(map[string]map[string]string{"SingleTap": {"x": "number", "y": "private123", "raw": "secret"}, "secret-name": {"x": "number"}})
	if s != "SingleTap{x:number}" || strings.Contains(s, "secret") {
		t.Fatal(s)
	}
}
