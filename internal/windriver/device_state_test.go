package windriver

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestDeviceTransitionSafetyGuards(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		change  func(*deviceGuard)
	}{
		{"missing", false, func(g *deviceGuard) { g.Count = 0 }},
		{"duplicates", false, func(g *deviceGuard) { g.Count = 2 }},
		{"customized package", false, func(g *deviceGuard) { g.Owned = false }},
		{"primary", false, func(g *deviceGuard) { g.Primary = true }},
		{"uncorrelated source", false, func(g *deviceGuard) { g.SourceIdentified = false }},
		{"no physical", false, func(g *deviceGuard) { g.PhysicalAttached = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := deviceGuard{Count: 1, Owned: true, PhysicalAttached: true, SourceIdentified: true}
			tt.change(&g)
			if err := validateDeviceTransition(g, tt.enabled); err == nil {
				t.Fatal("unsafe transition allowed")
			}
		})
	}
	g := deviceGuard{Count: 1, Owned: true, PhysicalAttached: true, SourceIdentified: true}
	if err := validateDeviceTransition(g, false); err != nil {
		t.Fatal(err)
	}
	g.Attached = true
	if err := validateDeviceTransition(g, false); err != nil {
		t.Fatal("dedicated attached nonprimary adapter must be disabled directly", err)
	}
	if err := validateDeviceTransition(g, true); err != nil {
		t.Fatal("enable refused existing active nonprimary adapter", err)
	}
}
func TestSingleMonitorConfiguration(t *testing.T) {
	valid := []byte("<vdd_settings><monitors><count>1</count></monitors></vdd_settings>")
	if err := validateSingleMonitorConfiguration(valid); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{strings.Replace(string(valid), ">1<", ">2<", 1), "<vdd_settings/>", "<vdd_settings><monitors><count>1</count><count>1</count></monitors></vdd_settings>", "<vdd_settings><monitors><count>1</count></monitors><monitors><count>1</count></monitors></vdd_settings>", "<bad>"} {
		if err := validateSingleMonitorConfiguration([]byte(s)); err == nil {
			t.Fatalf("unsafe configuration accepted: %s", s)
		}
	}
}
func TestDeviceStateRepeatedEnableDisableCycles(t *testing.T) {
	enabled := deviceState("ROOT\\MTTVDD\\0000", 8, 0)
	disabled := deviceState(enabled.Instance, 0x400, 22)
	if !enabled.Present || !enabled.Enabled || enabled.Disabled || !disabled.Disabled || disabled.Enabled {
		t.Fatal("invalid native state classification")
	}
	state := disabled
	for i := 0; i < 3; i++ {
		if deviceNeedsChange(state, false) {
			t.Fatal("disabled state not idempotent")
		}
		if !deviceNeedsChange(state, true) {
			t.Fatal("enable skipped")
		}
		state = enabled
		if deviceNeedsChange(state, true) {
			t.Fatal("enabled state not idempotent")
		}
		if !deviceNeedsChange(state, false) {
			t.Fatal("disable skipped")
		}
		state = disabled
	}
	for _, s := range []DeviceState{deviceState("x", 0, 0), deviceState("x", 8|0x400, 22), deviceState("x", 8|0x400, 10), deviceState("x", 8, 10)} {
		if s.Enabled {
			t.Fatal("problematic or stopped device reported enabled")
		}
	}
}

func TestDisplayDriverClassKeyIdentity(t *testing.T) {
	suffix := `{4d36e968-e325-11ce-bfc1-08002be10318}\0002`
	for _, root := range []string{`\Registry\Machine\SYSTEM\CurrentControlSet\Control\Class\`, `\Registry\Machine\SYSTEM\ControlSet001\Control\Class\`} {
		if got := displayDriverClassKey(root + suffix); got != suffix {
			t.Fatalf("wrong key %q", got)
		}
	}
	for _, path := range []string{`\Registry\Machine\SYSTEM\ControlSetFake\Control\Class\` + suffix, `\Registry\Machine\SYSTEM\CurrentControlSet\Control\Video\` + suffix, `\Registry\User\SYSTEM\CurrentControlSet\Control\Class\` + suffix} {
		if displayDriverClassKey(path) != "" {
			t.Fatal("accepted unrelated registry key")
		}
	}
}

func TestNativeDeviceErrorPreservesConfigurationManagerStatus(t *testing.T) {
	err := fmt.Errorf("adapter transition: %w", &NativeDeviceError{Operation: "CM_Disable_DevNode", Code: 0x17})
	var native *NativeDeviceError
	if !errors.As(err, &native) || native.Operation != "CM_Disable_DevNode" || native.Code != 0x17 || !strings.Contains(native.Error(), "0x17") {
		t.Fatal("native transition status was lost")
	}
}

func TestPublishedINFAndClassKeyValidation(t *testing.T) {
	for _, name := range []string{"oem1.inf", "OEM23.INF", "oem000.inf"} {
		if !validPublishedINF(name) {
			t.Fatalf("valid INF rejected %q", name)
		}
	}
	for _, name := range []string{"oem.inf", "MttVDD.inf", `..\oem1.inf`, `C:\Windows\INF\oem1.inf`, "oem1.inf/other", "oem1.inf:stream", "oem-1.inf", "oem1.inf ", `oem1.inf\`} {
		if validPublishedINF(name) {
			t.Fatalf("unsafe INF accepted %q", name)
		}
	}
	key := `{4d36e968-e325-11ce-bfc1-08002be10318}\0001`
	if !validDisplayDriverClassKey(key) {
		t.Fatal("valid display key rejected")
	}
	for _, bad := range []string{key + `\other`, strings.Replace(key, "0001", "..", 1), strings.Replace(key, "e968", "e967", 1), strings.Replace(key, "0001", "abcd", 1)} {
		if validDisplayDriverClassKey(bad) {
			t.Fatalf("unsafe driver key accepted %q", bad)
		}
	}
}
func TestInstalledPackageMatchesPinnedArchive(t *testing.T) {
	data, err := os.ReadFile("assets/VirtualDisplayDriver-x86.Driver.Only.zip")
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := packageFiles(data, ArchiveSHA256)
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string][]byte{}
	for _, name := range []string{"MttVDD.inf", "MttVDD.dll", "mttvdd.cat"} {
		actual[name] = append([]byte(nil), pinned[name]...)
	}
	if err := validateInstalledPackageFiles(actual, pinned); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MttVDD.inf", "MttVDD.dll", "mttvdd.cat"} {
		original := actual[name]
		changed := append([]byte(nil), original...)
		changed[len(changed)/2] ^= 1
		actual[name] = changed
		if validateInstalledPackageFiles(actual, pinned) == nil {
			t.Fatalf("modified installed %s accepted", name)
		}
		actual[name] = original
	}
	delete(actual, "MttVDD.dll")
	if validateInstalledPackageFiles(actual, pinned) == nil {
		t.Fatal("missing installed DLL accepted")
	}
}

func TestOwnedAdapterCanEnableWithoutPhysicalDisplay(t *testing.T) {
	g := deviceGuard{Count: 1, Owned: true, Primary: true}
	if err := validateDeviceTransition(g, true); err != nil {
		t.Fatal(err)
	}
	if validateDeviceTransition(g, false) == nil {
		t.Fatal("sole adapter disabled")
	}
	g.Owned = false
	if validateDeviceTransition(g, true) == nil {
		t.Fatal("unowned adapter enabled")
	}
}
