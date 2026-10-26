package camerapreview

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLocalTokenPreview(t *testing.T) {
	s, e := New()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if !strings.HasPrefix(s.URL(), "http://127.0.0.1:") {
		t.Fatal(s.URL())
	}
	r, e := http.Get(s.URL())
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	r, e = http.Get(strings.TrimSuffix(s.URL(), s.token+"/"))
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal(r.StatusCode)
	}
	r, e = http.Get(s.OBSURL())
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if !bytes.Contains(b, []byte("640px")) {
		t.Fatal(string(b))
	}
	if s.Update([]byte("bad")) == nil {
		t.Fatal("invalid JPEG accepted")
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 640, 480)), nil)
	if e = s.Update(buf.Bytes()); e != nil {
		t.Fatal(e)
	}
}
