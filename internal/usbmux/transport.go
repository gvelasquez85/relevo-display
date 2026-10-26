package usbmux

import "fmt"

// ChooseTransport selects metadata only, without connecting or authenticating.
// Network discovery does not imply that a USB tunnel or a secure LAN session
// exists. Callers must implement and authenticate the appropriate transport.
// There is deliberately no implicit fallback between USB and Network.
func ChooseTransport(ds []Device, id uint32, transport string) (Device, error) {
	if transport != "USB" && transport != "Network" {
		return Device{}, fmt.Errorf("unsupported transport %q: select USB or Network explicitly", transport)
	}
	var matches []Device
	for _, d := range ds {
		if d.Connection == transport && (id == 0 || d.ID == id) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return Device{}, fmt.Errorf("no matching %s device", transport)
	}
	if len(matches) != 1 {
		return Device{}, fmt.Errorf("multiple %s devices: select a device ID", transport)
	}
	return matches[0], nil
}
