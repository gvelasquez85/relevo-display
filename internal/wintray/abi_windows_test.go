//go:build windows && amd64

package wintray

import (
	"testing"
	"unsafe"
)

func TestNativeABISizes(t *testing.T) {
	for name, v := range map[string]struct{ got, want uintptr }{"NOTIFYICONDATAW": {unsafe.Sizeof(nid{}), 976}, "WNDCLASSEXW": {unsafe.Sizeof(wc{}), 80}, "MSG": {unsafe.Sizeof(msg{}), 48}, "SHELLEXECUTEINFOW": {unsafe.Sizeof(shellExecuteInfo{}), 112}} {
		if v.got != v.want {
			t.Errorf("%s size %d want%d", name, v.got, v.want)
		}
	}
}
