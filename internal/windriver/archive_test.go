package windriver

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestPinnedPackage(t *testing.T) {
	b, e := os.ReadFile("assets/VirtualDisplayDriver-x86.Driver.Only.zip")
	if e != nil {
		t.Fatal(e)
	}
	files, e := packageFiles(b, ArchiveSHA256)
	if e != nil {
		t.Fatal(e)
	}
	if len(files) != 4 || len(files["MttVDD.inf"]) != 4132 {
		t.Fatal("unexpected upstream package")
	}
	b[len(b)/2] ^= 1
	if _, e = packageFiles(b, ArchiveSHA256); e == nil {
		t.Fatal("accepted modified upstream archive")
	}
}

func TestArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, extra := range []struct {
		name string
		mode os.FileMode
	}{
		{"VirtualDisplayDriver/../escape", 0644},
		{"VirtualDisplayDriver/MttVDD.inf", 0644},
		{"VirtualDisplayDriver/unknown", 0644},
		{"VirtualDisplayDriver/MttVDD.dll", os.ModeSymlink | 0777},
	} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		for _, name := range []string{"mttvdd.cat", "MttVDD.dll", "MttVDD.inf", "vdd_settings.xml"} {
			w, _ := z.Create("VirtualDisplayDriver/" + name)
			w.Write([]byte("x"))
		}
		h := &zip.FileHeader{Name: extra.name}
		h.SetMode(extra.mode)
		w, _ := z.CreateHeader(h)
		w.Write([]byte("x"))
		z.Close()
		if _, e := packageFiles(b.Bytes(), fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))); e == nil {
			t.Fatalf("accepted unsafe entry %s", extra.name)
		}
	}
}
func TestRejectIncompletePackage(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	z.Close()
	if _, e := packageFiles(b.Bytes(), fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))); e == nil {
		t.Fatal("accepted incomplete archive")
	}
}
