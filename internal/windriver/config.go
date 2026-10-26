package windriver

import (
	"bytes"
	"fmt"
)

// ipadConfiguration changes only the unsigned runtime settings, never driver binaries.
func ipadConfiguration(original []byte) ([]byte, error) {
	closeTag := []byte("</resolutions>")
	if bytes.Count(original, closeTag) != 1 {
		return nil, fmt.Errorf("invalid pinned resolution settings")
	}
	modes := []byte(`<resolution><width>1024</width><height>768</height><refresh_rate>60</refresh_rate></resolution>
<resolution><width>1280</width><height>960</height><refresh_rate>60</refresh_rate></resolution>
<resolution><width>1600</width><height>1200</height><refresh_rate>60</refresh_rate></resolution>
<resolution><width>2048</width><height>1536</height><refresh_rate>60</refresh_rate></resolution>
`)
	return bytes.Replace(original, closeTag, append(modes, closeTag...), 1), nil
}

// validateOwnedFiles permits retrying an interrupted install only when every
// file is still an exact pinned asset, or our generated settings with its exact
// original backup. Unknown files and customized binary/configuration data fail.
func validateOwnedFiles(actual, pinned map[string][]byte) error {
	original, ok := pinned["vdd_settings.xml"]
	if !ok {
		return fmt.Errorf("missing pinned configuration")
	}
	desired, e := ipadConfiguration(original)
	if e != nil {
		return e
	}
	const backup = "vdd_settings.xml.yam-v03-backup"
	if len(actual) != len(pinned) && len(actual) != len(pinned)+1 {
		return fmt.Errorf("unexpected driver directory contents")
	}
	for name, want := range pinned {
		got, ok := actual[name]
		if !ok {
			return fmt.Errorf("missing driver file %s", name)
		}
		if name == "vdd_settings.xml" && bytes.Equal(got, desired) {
			if !bytes.Equal(actual[backup], original) {
				return fmt.Errorf("modified configuration requires exact original backup")
			}
			continue
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("driver file differs from owned package: %s", name)
		}
	}
	for name, b := range actual {
		if _, ok := pinned[name]; ok {
			continue
		}
		if name != backup || !bytes.Equal(b, original) {
			return fmt.Errorf("unexpected or modified driver file %s", name)
		}
	}
	return nil
}
