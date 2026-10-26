package main

import (
	"encoding/json"
	"testing"
)

func TestExportDistinguishesOldSessionFromRunningVersion(t *testing.T) {
	text, err := diagnosticForExport([]byte(`{"Version":"0.12.3-experimental","Error":"activation failed"}`), diagnosticIdentity{Version: "0.12.5-experimental", ProcessID: 123, ExecutableSHA256: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Version    string
		ExportedBy diagnosticIdentity
		Error      string
	}
	if err = json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatal(err)
	}
	if result.Version != "0.12.3-experimental" || result.ExportedBy.Version != "0.12.5-experimental" || result.ExportedBy.ProcessID != 123 || result.Error != "activation failed" {
		t.Fatalf("ambiguous exported identity: %+v", result)
	}
}
func TestExportRejectsInvalidSessionJSON(t *testing.T) {
	for _, b := range []string{"invalid", "null", "[]"} {
		if _, err := diagnosticForExport([]byte(b), diagnosticIdentity{}); err == nil {
			t.Fatalf("accepted invalid record %q", b)
		}
	}
}
