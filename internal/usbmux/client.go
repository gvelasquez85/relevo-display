// Package usbmux speaks to the local Apple/usbmuxd service, without drivers or pairing changes.
package usbmux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"time"

	"howett.net/plist"
)

type Client struct {
	Endpoint string
	// VerifyConnection runs before any USB protocol bytes are exchanged. The
	// privileged sign-in broker uses it to authenticate the local Apple service.
	VerifyConnection func(net.Conn) error
}
type Device struct {
	ID         uint32
	Connection string
}

func (c *Client) dial() (net.Conn, error) {
	ep := c.Endpoint
	if ep == "" {
		if runtime.GOOS == "windows" {
			ep = "tcp://127.0.0.1:27015"
		} else {
			ep = "unix:///var/run/usbmuxd"
		}
	}
	var network, address string
	switch {
	case strings.HasPrefix(ep, "tcp://"):
		network = "tcp"
		address = strings.TrimPrefix(ep, "tcp://")
	case strings.HasPrefix(ep, "unix://"):
		network = "unix"
		address = strings.TrimPrefix(ep, "unix://")
	default:
		return nil, errors.New("endpoint must use unix:// or tcp://")
	}
	conn, e := net.DialTimeout(network, address, 3*time.Second)
	if e != nil {
		return nil, fmt.Errorf("local usbmux service: %w", e)
	}
	if c.VerifyConnection != nil {
		if err := c.VerifyConnection(conn); err != nil {
			conn.Close()
			return nil, err
		}
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	return conn, nil
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func control(conn net.Conn, request map[string]interface{}) (map[string]interface{}, error) {
	request["ClientVersionString"] = "Relevo-0.6"
	request["ProgName"] = "RelevoDisplay"
	request["kLibUSBMuxVersion"] = uint64(3)
	b, e := plist.Marshal(request, plist.XMLFormat)
	if e != nil {
		return nil, e
	}
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h, uint32(len(b)+16))
	binary.LittleEndian.PutUint32(h[4:], 1)
	binary.LittleEndian.PutUint32(h[8:], 8)
	binary.LittleEndian.PutUint32(h[12:], 1)
	if e = writeAll(conn, append(h, b...)); e != nil {
		return nil, e
	}
	if _, e = io.ReadFull(conn, h); e != nil {
		return nil, e
	}
	n := binary.LittleEndian.Uint32(h)
	if n < 16 || n > 1024*1024 {
		return nil, errors.New("invalid usbmux response length")
	}
	if binary.LittleEndian.Uint32(h[4:]) != 1 || binary.LittleEndian.Uint32(h[8:]) != 8 || binary.LittleEndian.Uint32(h[12:]) != 1 {
		return nil, errors.New("unexpected usbmux response header")
	}
	b = make([]byte, n-16)
	if _, e = io.ReadFull(conn, b); e != nil {
		return nil, e
	}
	var response map[string]interface{}
	if _, e = plist.Unmarshal(b, &response); e != nil {
		return nil, e
	}
	return response, nil
}

func (c *Client) Devices() ([]Device, error) {
	conn, e := c.dial()
	if e != nil {
		return nil, e
	}
	defer conn.Close()
	r, e := control(conn, map[string]interface{}{"MessageType": "ListDevices"})
	if e != nil {
		return nil, e
	}
	list, ok := r["DeviceList"].([]interface{})
	if !ok {
		return nil, errors.New("usbmux did not return DeviceList")
	}
	var devices []Device
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id, ok := m["DeviceID"].(uint64)
		if !ok || id > 0xffffffff {
			continue
		}
		props, _ := m["Properties"].(map[string]interface{})
		kind, _ := props["ConnectionType"].(string)
		devices = append(devices, Device{ID: uint32(id), Connection: kind})
	}
	return devices, nil
}

func Choose(ds []Device, id uint32) (Device, error) {
	var matches []Device
	for _, d := range ds {
		if (id != 0 && d.ID == id) || (id == 0 && d.Connection == "USB") {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return Device{}, errors.New("no matching USB device: connect and unlock the iPad, trust this computer, and open the compatible iPad app")
	}
	if len(matches) != 1 {
		return Device{}, errors.New("multiple USB devices: select one with --device ID")
	}
	return matches[0], nil
}

func (c *Client) Connect(id uint32, port uint16) (net.Conn, error) {
	conn, e := c.dial()
	if e != nil {
		return nil, e
	}
	swapped := uint16(port<<8) | port>>8
	r, e := control(conn, map[string]interface{}{"MessageType": "Connect", "DeviceID": uint64(id), "PortNumber": uint64(swapped)})
	if e != nil {
		conn.Close()
		return nil, e
	}
	code, ok := r["Number"].(uint64)
	if !ok || code != 0 {
		conn.Close()
		return nil, fmt.Errorf("iPad USB connection refused (usbmux code %v): close the original desktop server and keep the compatible app open on the iPad", r["Number"])
	}
	conn.SetDeadline(time.Time{})
	return conn, nil
}
