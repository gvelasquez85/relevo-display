//go:build linux

package desktop

import (
	"github.com/godbus/dbus/v5"
	"testing"
)

func TestPortalResponseRejectsCancellation(t *testing.T) {
	if _, e := portalResponse([]interface{}{uint32(1), map[string]dbus.Variant{}}); e == nil {
		t.Fatal("cancellation accepted")
	}
}
func TestPortalResponseMalformed(t *testing.T) {
	for _, b := range [][]interface{}{nil, {"bad", map[string]dbus.Variant{}}, {uint32(0), "bad"}} {
		if _, e := portalResponse(b); e == nil {
			t.Fatal("malformed response accepted")
		}
	}
}
func TestPortalSourceRequiresVirtual(t *testing.T) {
	if _, e := portalSourceType("extend", 1); e == nil {
		t.Fatal("silently mirrored")
	}
	if n, e := portalSourceType("extend", 5); e != nil || n != 4 {
		t.Fatal(n, e)
	}
}
func TestPortalStreams(t *testing.T) {
	v := dbus.MakeVariant([]struct {
		Node  uint32
		Props map[string]dbus.Variant
	}{{42, map[string]dbus.Variant{}}})
	n, e := portalNode(v)
	if e != nil || n != 42 {
		t.Fatal(n, e)
	}
}

func TestHyprMonitorNames(t *testing.T) {
	names, e := hyprMonitorNames([]byte(`[{"name":"DP-1"},{"name":"relevo-123"}]`))
	if e != nil || !names["relevo-123"] || names["missing"] {
		t.Fatal(names, e)
	}
	if _, e := hyprMonitorNames([]byte(`not json`)); e == nil {
		t.Fatal("invalid monitor list accepted")
	}
}

func TestPortalGstSelectedResolution(t *testing.T) {
	args := portalGstArgsSize(42, 5, 1024, 768)
	found := false
	for _, arg := range args {
		if arg == "video/x-raw,width=1024,height=768,pixel-aspect-ratio=1/1,framerate=5/1" {
			found = true
		}
	}
	if !found {
		t.Fatal(args)
	}
}
