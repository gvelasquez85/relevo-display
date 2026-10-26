package nativeapp

import "testing"

func TestLocalEndpoint(t *testing.T) {
	for _, s := range []string{"", "unix:///tmp/test.sock", "tcp://127.0.0.1:27015", "tcp://[::1]:27015"} {
		if e := localEndpoint(s); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []string{"tcp://192.168.1.1:27015", "tcp://localhost:27015", "https://127.0.0.1", "tcp://0.0.0.0:27015"} {
		if e := localEndpoint(s); e == nil {
			t.Fatal(s)
		}
	}
}
