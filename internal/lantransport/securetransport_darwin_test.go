//go:build darwin

package lantransport

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This tests the exact iOS TLS source with macOS SecureTransport, not an iPad.
func TestSecureTransportInteroperability(t *testing.T) {
	if _, e := exec.LookPath("xcrun"); e != nil {
		t.Skip("Apple compiler unavailable; SecureTransport interoperability not run")
	}
	if e := exec.Command("xcrun", "--find", "clang").Run(); e != nil {
		t.Skip("Apple clang unavailable; SecureTransport interoperability not run")
	}
	if e := exec.Command("xcrun", "--sdk", "macosx", "--show-sdk-path").Run(); e != nil {
		t.Skip("macOS SDK unavailable; SecureTransport interoperability not run")
	}
	binary := filepath.Join(t.TempDir(), "securetransport")
	build := exec.Command("xcrun", "clang", "-fobjc-arc", "-fmodules", "-Wno-deprecated-declarations", "-framework", "Foundation", "-framework", "Security", "-I../../clients/ios/Sources", "testdata/securetransport.m", "../../clients/ios/Sources/RVLANTransport.m", "-o", binary)
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("SecureTransport harness compile failed: %v\n%s", e, output)
	}
	for _, scenario := range []string{"valid", "valid held open", "wrong pin", "wrong token"} {
		t.Run(scenario, func(t *testing.T) {
			s, e := newServer("127.0.0.1:0", 10*time.Second)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			pin, token := credentials(t, s)
			if scenario == "wrong pin" {
				if pin[0] == '0' {
					pin = "1" + pin[1:]
				} else {
					pin = "0" + pin[1:]
				}
			}
			if scenario == "wrong token" {
				token[0] ^= 255
			}
			_, port, _ := net.SplitHostPort(s.Addr().String())
			payload, _ := json.Marshal(map[string]any{"pin": pin, "token": hex.EncodeToString(token), "port": port, "reject": scenario != "valid" && scenario != "valid held open"})
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, binary)
			child.Stdin = bytes.NewReader(payload)
			serverDone := make(chan error, 1)
			go func() {
				c, e := s.Accept(ctx)
				if e != nil {
					serverDone <- e
					return
				}
				if scenario != "valid" && scenario != "valid held open" {
					serverDone <- ErrUnavailable
					return
				}
				defer c.Close()
				// Deliberately fragment TLS application records before the prefix is complete.
				for _, part := range []string{"R", "V", "L1"} {
					if _, e = c.Write([]byte(part)); e != nil {
						serverDone <- e
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				prefix := make([]byte, 4)
				if _, e = io.ReadFull(c, prefix); e != nil || string(prefix) != "RVL1" {
					serverDone <- ErrUnavailable
					return
				}
				// Stall the receiver to exercise partial/nonblocking SSLWrite under pressure.
				time.Sleep(150 * time.Millisecond)
				data := make([]byte, 512*1024)
				if _, e = io.ReadFull(c, data); e != nil {
					serverDone <- e
					return
				}
				for i, b := range data {
					if b != byte(i%251) {
						serverDone <- ErrUnavailable
						return
					}
				}
				for offset := 0; offset < len(data); offset += 997 {
					end := offset + 997
					if end > len(data) {
						end = len(data)
					}
					if _, e = c.Write(data[offset:end]); e != nil {
						serverDone <- e
						return
					}
				}
				if scenario == "valid held open" {
					time.Sleep(500 * time.Millisecond)
				}
				serverDone <- nil
			}()
			output, e := child.CombinedOutput()
			// Never print subprocess input or pairing credentials, even on failure.
			if e != nil {
				s.Close()
				t.Fatalf("SecureTransport client failed (%s): %v", scenario, e)
			}
			if scenario == "valid" || scenario == "valid held open" {
				if string(output) != "OK\n" {
					t.Fatal("missing success marker")
				}
				if e := <-serverDone; e != nil {
					t.Fatal("authenticated stream failed", e)
				}
			} else {
				if string(output) != "REJECTED\n" {
					t.Fatal("client accepted invalid pairing")
				}
				s.Close()
				<-serverDone
			}
		})
	}
}
