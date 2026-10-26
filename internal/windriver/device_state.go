package windriver

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// DeviceState distinguishes an inactive desktop path from an actually stopped
// Plug and Play adapter. Enabled requires DN_STARTED without a device problem.
var ErrDeviceElevationRequired = errors.New("cambiar el adaptador virtual requiere permisos de administrador")

// NativeDeviceError preserves the CONFIGRET returned by a device transition.
// A GUI parent can convey this numeric status through its elevated helper's
// exit code without creating a privileged diagnostic file.
type NativeDeviceError struct {
	Operation string
	Code      uint32
}

func (e *NativeDeviceError) Error() string { return fmt.Sprintf("%s status %#x", e.Operation, e.Code) }

type DeviceState struct {
	Present, Enabled, Disabled bool
	Problem, Status            uint32
	Instance                   string
}

func deviceState(instance string, status, problem uint32) DeviceState {
	return DeviceState{Present: true, Enabled: status&8 != 0 && status&0x400 == 0 && problem == 0, Disabled: status&0x400 != 0 && problem == 22, Status: status, Problem: problem, Instance: instance}
}

type deviceGuard struct {
	Count                               int
	Owned                               bool
	Primary, Attached, PhysicalAttached bool
	SourceIdentified                    bool
}

func validateDeviceTransition(g deviceGuard, enabled bool) error {
	if g.Count != 1 {
		return fmt.Errorf("se requiere exactamente un adaptador Root\\MttVDD; encontrados %d", g.Count)
	}
	if !g.Owned {
		return fmt.Errorf("el controlador o su configuración no coincide con el paquete de Relevo")
	}
	if g.Primary {
		return fmt.Errorf("no se puede cambiar el adaptador de la pantalla principal")
	}
	if !g.PhysicalAttached {
		return fmt.Errorf("no se puede cambiar el adaptador sin otra pantalla física activa")
	}
	if !enabled && !g.SourceIdentified {
		return fmt.Errorf("no se pudo correlacionar el adaptador virtual con sus pantallas de Windows")
	}
	return nil
}
func validateSingleMonitorConfiguration(b []byte) error {
	var config struct {
		XMLName  xml.Name `xml:"vdd_settings"`
		Monitors []struct {
			Counts []int `xml:"count"`
		} `xml:"monitors"`
	}
	if err := xml.Unmarshal(b, &config); err != nil {
		return fmt.Errorf("invalid driver configuration: %w", err)
	}
	if len(config.Monitors) != 1 || len(config.Monitors[0].Counts) != 1 || config.Monitors[0].Counts[0] != 1 {
		return fmt.Errorf("virtual adapter must have exactly one configured monitor")
	}
	return nil
}

func deviceNeedsChange(state DeviceState, enabled bool) bool {
	return !(enabled && state.Enabled || !enabled && state.Disabled)
}

// Class keys are compared by the exact class-GUID\instance suffix. Windows
// may expose CurrentControlSet or the concrete ControlSet00N alias.
func displayDriverClassKey(path string) string {
	path = strings.ToLower(path)
	const root = `\registry\machine\system\`
	if !strings.HasPrefix(path, root) {
		return ""
	}
	suffix := strings.TrimPrefix(path, root)
	split := strings.IndexByte(suffix, '\\')
	if split < 0 {
		return ""
	}
	set := suffix[:split]
	if set != "currentcontrolset" {
		if len(set) != 13 || !strings.HasPrefix(set, "controlset") {
			return ""
		}
		for _, c := range set[10:] {
			if c < '0' || c > '9' {
				return ""
			}
		}
	}
	const class = `\control\class\`
	suffix = suffix[split:]
	if !strings.HasPrefix(suffix, class) {
		return ""
	}
	return strings.TrimPrefix(suffix, class)
}

func validPublishedINF(name string) bool {
	name = strings.ToLower(name)
	if !strings.HasPrefix(name, "oem") || !strings.HasSuffix(name, ".inf") {
		return false
	}
	number := name[3 : len(name)-4]
	if len(number) == 0 || len(number) > 10 {
		return false
	}
	for _, c := range number {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func validDisplayDriverClassKey(key string) bool {
	const prefix = `{4d36e968-e325-11ce-bfc1-08002be10318}\`
	key = strings.ToLower(key)
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	instance := strings.TrimPrefix(key, prefix)
	if len(instance) != 4 {
		return false
	}
	for _, c := range instance {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func validateInstalledPackageFiles(actual, pinned map[string][]byte) error {
	for _, name := range []string{"MttVDD.inf", "MttVDD.dll", "mttvdd.cat"} {
		expected, ok := pinned[name]
		if !ok || len(expected) == 0 {
			return fmt.Errorf("missing pinned installed driver asset %s", name)
		}
		data, ok := actual[name]
		if !ok || !bytes.Equal(data, expected) {
			return fmt.Errorf("installed driver asset differs from Relevo package: %s", name)
		}
	}
	return nil
}
