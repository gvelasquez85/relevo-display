package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"relevo/internal/windriver"
)

// Session logs can predate the running program. Keep their original version
// and identify the exporter separately so old logs never imply a new run.
type diagnosticIdentity struct {
	Version                 string
	ProcessID               int
	ExecutableName          string
	ExecutableSHA256        string
	MonitorDeviceState      *windriver.DeviceState `json:",omitempty"`
	MonitorDeviceStateError string                 `json:",omitempty"`
}

func currentDiagnosticIdentity() diagnosticIdentity {
	id := diagnosticIdentity{Version: version, ProcessID: os.Getpid()}
	state, stateErr := windriver.ReadDeviceState(context.Background())
	if stateErr != nil {
		id.MonitorDeviceStateError = stateErr.Error()
	} else {
		id.MonitorDeviceState = &state
	}
	path, err := os.Executable()
	if err != nil {
		return id
	}
	id.ExecutableName = filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return id
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err == nil {
		id.ExecutableSHA256 = hex.EncodeToString(h.Sum(nil))
	}
	return id
}
func diagnosticForExport(session []byte, id diagnosticIdentity) (string, error) {
	var record map[string]json.RawMessage
	if err := json.Unmarshal(session, &record); err != nil {
		return "", fmt.Errorf("leer diagnóstico guardado: %w", err)
	}
	if record == nil {
		return "", fmt.Errorf("diagnóstico guardado vacío")
	}
	identity, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	record["ExportedBy"] = identity
	b, err := json.MarshalIndent(record, "", "  ")
	return string(b), err
}
