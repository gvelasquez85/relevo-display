package windriver

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

const ArchiveSHA256 = "e24210692b442b39af763536330ce78b423f19342b7a7792c26de3944e418b3a"

// packageFiles validates the entire immutable upstream archive before any writes.
func packageFiles(data []byte, digest string) (map[string][]byte, error) {
	if fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return nil, fmt.Errorf("driver archive SHA256 mismatch")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"mttvdd.cat": true, "MttVDD.dll": true, "MttVDD.inf": true, "vdd_settings.xml": true}
	out := map[string][]byte{}
	for _, f := range z.File {
		if f.Name == "VirtualDisplayDriver/" && f.FileInfo().IsDir() {
			continue
		}
		const prefix = "VirtualDisplayDriver/"
		if len(f.Name) <= len(prefix) || f.Name[:len(prefix)] != prefix {
			return nil, fmt.Errorf("unexpected archive path %q", f.Name)
		}
		name := f.Name[len(prefix):]
		if !allowed[name] || !f.Mode().IsRegular() || f.UncompressedSize64 > 4<<20 {
			return nil, fmt.Errorf("unsafe archive entry %q", f.Name)
		}
		if _, ok := out[name]; ok {
			return nil, fmt.Errorf("duplicate archive entry")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(io.LimitReader(r, 4<<20+1))
		r.Close()
		if e != nil {
			return nil, e
		}
		if len(b) > 4<<20 {
			return nil, fmt.Errorf("oversized entry")
		}
		out[name] = b
	}
	if len(out) != len(allowed) {
		return nil, fmt.Errorf("incomplete driver archive")
	}
	return out, nil
}
