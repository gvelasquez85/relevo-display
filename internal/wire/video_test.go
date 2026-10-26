package wire

import (
	"bytes"
	"errors"
	"io"
	"relevo/internal/pattern"
	"testing"
)

type countingWriter struct {
	bytes.Buffer
	calls int
}

func (w *countingWriter) Write(p []byte) (int, error) { w.calls++; return w.Buffer.Write(p) }

type failingVideoWriter struct{}

func (failingVideoWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestVideoWriterMatchesLegacyAndReusesBuffer(t *testing.T) {
	var v VideoWriter
	for _, jpg := range [][]byte{{1, 2, 3}, bytes.Repeat([]byte{7}, 10000), {9}} {
		var want bytes.Buffer
		if err := Write(&want, Frame{Type: 108, Payload: pattern.VideoPayload(jpg)}); err != nil {
			t.Fatal(err)
		}
		var got countingWriter
		if err := v.WriteJPEG(&got, jpg); err != nil {
			t.Fatal(err)
		}
		if got.calls != 1 || !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("calls=%d bytes differ=%v", got.calls, !bytes.Equal(got.Bytes(), want.Bytes()))
		}
		old := &v.buffer[0]
		var partial chunkWriter
		if err := v.WriteJPEG(&partial, jpg); err != nil || !bytes.Equal(partial.Bytes(), want.Bytes()) {
			t.Fatal("partial write", err)
		}
		if old != &v.buffer[0] {
			t.Fatal("buffer not reused")
		}
	}
	if err := v.WriteJPEG(failingVideoWriter{}, []byte{1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := v.WriteJPEG(zeroWriter{}, []byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := v.WriteJPEG(io.Discard, make([]byte, MaxPayload-12)); err == nil {
		t.Fatal("oversize accepted")
	}
}
func BenchmarkVideoFraming(b *testing.B) {
	jpg := bytes.Repeat([]byte{7}, 100000)
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Write(io.Discard, Frame{Type: 108, Payload: pattern.VideoPayload(jpg)})
		}
	})
	b.Run("reused", func(b *testing.B) {
		var v VideoWriter
		v.WriteJPEG(io.Discard, jpg)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.WriteJPEG(io.Discard, jpg)
		}
	})
}
