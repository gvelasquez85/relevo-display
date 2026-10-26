package usbmux

import "testing"

func TestChooseTransportExplicitOnly(t *testing.T) {
	ds := []Device{{ID: 1, Connection: "Network"}, {ID: 2, Connection: "USB"}}
	for _, tt := range []struct {
		name      string
		id        uint32
		transport string
		want      uint32
		bad       bool
	}{
		{"usb", 0, "USB", 2, false}, {"network", 0, "Network", 1, false},
		{"explicit network", 1, "Network", 1, false}, {"id cannot override transport", 1, "USB", 0, true},
		{"missing", 8, "Network", 0, true}, {"no auto", 0, "", 0, true},
		{"unknown", 0, "Bluetooth", 0, true}, {"case sensitive", 0, "network", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChooseTransport(ds, tt.id, tt.transport)
			if (err != nil) != tt.bad || (!tt.bad && got.ID != tt.want) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	if _, err := ChooseTransport([]Device{{ID: 1, Connection: "Network"}}, 0, "USB"); err == nil {
		t.Fatal("fell back to Network")
	}
	if _, err := ChooseTransport([]Device{{ID: 1, Connection: "USB"}}, 0, "Network"); err == nil {
		t.Fatal("fell back to USB")
	}
	if _, err := ChooseTransport([]Device{{ID: 1, Connection: "Network"}, {ID: 3, Connection: "Network"}}, 0, "Network"); err == nil {
		t.Fatal("selected ambiguous device")
	}
}
