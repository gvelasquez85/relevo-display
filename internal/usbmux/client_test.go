package usbmux

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"howett.net/plist"
)

func serve(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		c, e := l.Accept()
		if e == nil {
			defer c.Close()
			handle(c)
		}
	}()
	return "tcp://" + l.Addr().String()
}
func request(t *testing.T, c net.Conn) map[string]interface{} {
	t.Helper()
	h := make([]byte, 16)
	if _, e := io.ReadFull(c, h); e != nil {
		t.Error(e)
		return nil
	}
	n := binary.LittleEndian.Uint32(h)
	if n < 16 || n > 65536 {
		t.Error("request length")
		return nil
	}
	b := make([]byte, n-16)
	io.ReadFull(c, b)
	var d map[string]interface{}
	if _, e := plist.Unmarshal(b, &d); e != nil {
		t.Error(e)
	}
	return d
}
func response(c net.Conn, d interface{}) {
	b, _ := plist.Marshal(d, plist.XMLFormat)
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h, uint32(len(b)+16))
	binary.LittleEndian.PutUint32(h[4:], 1)
	binary.LittleEndian.PutUint32(h[8:], 8)
	binary.LittleEndian.PutUint32(h[12:], 1)
	c.Write(append(h, b...))
}
func TestDevices(t *testing.T) {
	ep := serve(t, func(c net.Conn) {
		d := request(t, c)
		if d["MessageType"] != "ListDevices" {
			t.Error(d)
		}
		response(c, map[string]interface{}{"DeviceList": []interface{}{map[string]interface{}{"DeviceID": uint64(9), "Properties": map[string]interface{}{"ConnectionType": "USB"}}}})
	})
	ds, e := (&Client{Endpoint: ep}).Devices()
	if e != nil || len(ds) != 1 || ds[0].ID != 9 || ds[0].Connection != "USB" {
		t.Fatalf("%+v %v", ds, e)
	}
}
func TestConnectPortEndianAndTunnel(t *testing.T) {
	ep := serve(t, func(c net.Conn) {
		d := request(t, c)
		if d["PortNumber"] != uint64(0xaf5b) || d["DeviceID"] != uint64(9) {
			t.Errorf("bad connect %+v", d)
		}
		response(c, map[string]interface{}{"MessageType": "Result", "Number": 0})
		c.Write([]byte("USB"))
	})
	c, e := (&Client{Endpoint: ep}).Connect(9, 23471)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	b := make([]byte, 3)
	if _, e = io.ReadFull(c, b); e != nil || string(b) != "USB" {
		t.Fatalf("%q %v", b, e)
	}
}
func TestConnectRefused(t *testing.T) {
	ep := serve(t, func(c net.Conn) {
		request(t, c)
		response(c, map[string]interface{}{"MessageType": "Result", "Number": 3})
	})
	c, e := (&Client{Endpoint: ep}).Connect(9, 23471)
	if c != nil || e == nil {
		t.Fatal("refusal accepted")
	}
}
func TestRejectOversizeBeforeAllocating(t *testing.T) {
	ep := serve(t, func(c net.Conn) {
		request(t, c)
		h := make([]byte, 16)
		binary.LittleEndian.PutUint32(h, 0xffffffff)
		c.Write(h)
	})
	if _, e := (&Client{Endpoint: ep}).Devices(); e == nil {
		t.Fatal("accepted oversized frame")
	}
}
func TestChooseUSBRejectsAmbiguous(t *testing.T) {
	d, e := Choose([]Device{{ID: 1, Connection: "Network"}, {ID: 2, Connection: "USB"}}, 0)
	if e != nil || d.ID != 2 {
		t.Fatal(d, e)
	}
	if _, e = Choose([]Device{{ID: 1, Connection: "USB"}, {ID: 2, Connection: "USB"}}, 0); e == nil {
		t.Fatal("ambiguous selection")
	}
	if _, e = Choose([]Device{{ID: 1, Connection: "USB"}}, 2); e == nil {
		t.Fatal("missing requested device")
	}
}
