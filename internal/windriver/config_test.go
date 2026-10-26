package windriver

import (
	"encoding/xml"
	"testing"
)

func TestIPadConfigPreservesExistingSettings(t *testing.T) {
	// A bounded fixture checks that options and existing modes survive insertion.
	original := []byte(`<vdd_settings><resolutions><resolution><width>800</width><height>600</height></resolution></resolutions><options><HardwareCursor>true</HardwareCursor></options></vdd_settings>`)
	b, e := ipadConfiguration(original)
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Modes []struct {
			W int `xml:"width"`
			H int `xml:"height"`
		} `xml:"resolutions>resolution"`
		Options struct {
			Cursor bool `xml:"HardwareCursor"`
		} `xml:"options"`
	}
	if e = xml.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	if len(v.Modes) != 5 || !v.Options.Cursor {
		t.Fatalf("settings lost: %+v", v)
	}
	for _, m := range v.Modes[1:] {
		if m.W*3 != m.H*4 {
			t.Fatal(m)
		}
	}
}

// Ensure configuration remains outside the signed embedded package.
func TestConfigRejectsMalformed(t *testing.T) {
	if _, e := ipadConfiguration([]byte("<settings/>")); e == nil {
		t.Fatal("accepted malformed settings")
	}
}

func TestInstallRetryOwnedDirectoryShapes(t *testing.T) {
	original := []byte(`<vdd_settings><resolutions></resolutions></vdd_settings>`)
	modified, e := ipadConfiguration(original)
	if e != nil {
		t.Fatal(e)
	}
	pinned := map[string][]byte{"MttVDD.dll": []byte("pinned dll"), "MttVDD.inf": []byte("pinned inf"), "mttvdd.cat": []byte("pinned cat"), "vdd_settings.xml": original}
	clone := func() map[string][]byte {
		m := map[string][]byte{}
		for k, v := range pinned {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name    string
		change  func(map[string][]byte)
		wantErr bool
	}{
		{"fresh pinned", func(m map[string][]byte) {}, false},
		{"backup saved before config replacement", func(m map[string][]byte) { m["vdd_settings.xml.yam-v03-backup"] = original }, false},
		{"migrated config retry after trust or pnputil failure", func(m map[string][]byte) {
			m["vdd_settings.xml"] = modified
			m["vdd_settings.xml.yam-v03-backup"] = original
		}, false},
		{"modified config missing backup", func(m map[string][]byte) { m["vdd_settings.xml"] = modified }, true},
		{"modified backup", func(m map[string][]byte) {
			m["vdd_settings.xml"] = modified
			m["vdd_settings.xml.yam-v03-backup"] = []byte("other")
		}, true},
		{"unowned extra file", func(m map[string][]byte) { m["other.xml"] = original }, true},
		{"different binary", func(m map[string][]byte) { m["MttVDD.dll"] = []byte("other") }, true},
		{"customized config", func(m map[string][]byte) { m["vdd_settings.xml"] = []byte("other") }, true},
		{"missing signed asset", func(m map[string][]byte) { delete(m, "mttvdd.cat"); m["unexpected"] = original }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := clone()
			tt.change(m)
			if e := validateOwnedFiles(m, pinned); (e != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%t", e, tt.wantErr)
			}
		})
	}
}
