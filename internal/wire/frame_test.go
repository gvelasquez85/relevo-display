package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
)

type chunkReader struct{ io.Reader }

func (r chunkReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

type chunkWriter struct{ bytes.Buffer }

func (w *chunkWriter) Write(p []byte) (int, error) {
	if len(p) > 2 {
		p = p[:2]
	}
	return w.Buffer.Write(p)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestGoldenAndFragmentation(t *testing.T) {
	f := Frame{Type: 0x01020304, Tag: 0x05060708, Payload: []byte{0xaa, 0xbb, 0xcc}}
	want, _ := hex.DecodeString("00000001010203040506070800000003aabbcc")
	var b chunkWriter
	if err := Write(&b, f); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), want) {
		t.Fatalf("got %x want %x", b.Bytes(), want)
	}
	got, err := Read(chunkReader{bytes.NewReader(want)})
	if err != nil || got.Type != f.Type || got.Tag != f.Tag || !bytes.Equal(got.Payload, f.Payload) {
		t.Fatalf("got %+v err %v", got, err)
	}
}
func TestInvalidFrames(t *testing.T) {
	for _, tc := range []struct{ name, hex string }{
		{"version", "00000002000000010000000000000000"},
		{"oversize", "00000001000000010000000002000001"},
		{"short header", "0000000100000001"},
		{"short payload", "0000000100000001000000000000000361"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := hex.DecodeString(tc.hex)
			if _, err := Read(bytes.NewReader(b)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if _, err := Read(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fatalf("empty read: %v", err)
	}
	if err := Write(zeroWriter{}, Frame{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("zero write: %v", err)
	}
	if err := Write(io.Discard, Frame{Payload: make([]byte, MaxPayload+1)}); err == nil {
		t.Fatal("oversize write accepted")
	}
}
func TestText(t *testing.T) {
	s := "Hola, 世界 👋"
	b := Text(s)
	if got, err := ParseText(b); err != nil || got != s {
		t.Fatalf("got %q err %v", got, err)
	}
	if !bytes.Equal(Text("é"), []byte{0, 0, 0, 2, 0xc3, 0xa9}) {
		t.Fatal("length must count UTF8 bytes")
	}
	for _, b := range [][]byte{nil, {0, 0, 0}, {0, 0, 0, 2, 'a'}, {0, 0, 0, 0, 'a'}} {
		if _, err := ParseText(b); err == nil {
			t.Fatalf("accepted %x", b)
		}
	}
	if got, err := ParseText(Text("")); err != nil || got != "" {
		t.Fatal("empty text", got, err)
	}
	if got, err := ParseText(Text(strings.Repeat("x", 1000))); err != nil || len(got) != 1000 {
		t.Fatal(err)
	}
}
