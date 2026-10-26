package session

import (
	"context"
	"howett.net/plist"
	"net"
	"relevo/internal/wire"
	"strings"
	"testing"
	"time"
)

func TestInputDiagnosticsCountsKnownNamesWithoutInjection(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		for _, name := range []string{"SingleTap", "SingleTap", "TwoFingerSingleTap", "SingleTap2nd", "Scrolled", "Dragged", "HoverEvent", "KeyboardEvent"} {
			b, _ := plist.Marshal(map[string]interface{}{"eventName": name, "x": 12.5, "y": 99.5, "privateText": "never log"}, plist.BinaryFormat)
			if wire.Write(p, wire.Frame{Type: 104, Payload: b}) != nil {
				return
			}
		}
		b, _ := plist.Marshal(map[string]interface{}{"eventName": "DeviceLocked"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 104, Payload: b})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var logs []string
	st, e := Run(ctx, h, Options{Duration: time.Second, FPS: 5, Log: func(s string) { logs = append(logs, s) }})
	if e == nil || st.InputEventCounts["SingleTap"] != 2 || len(st.InputEventCounts) != 7 {
		t.Fatal(st, e)
	}
	if st.InputEventSchemas["SingleTap"]["x"] != "number" || st.InputEventSchemas["SingleTap"]["privateText"] != "" {
		t.Fatal("unsafe or missing schema", st.InputEventSchemas)
	}
	for _, line := range logs {
		if strings.Contains(line, "never log") {
			t.Fatal("input payload logged")
		}
	}
}
