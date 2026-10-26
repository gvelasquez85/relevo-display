package medialab

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTone(t *testing.T) {
	b := Tone()
	if len(b) != 220544 || string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || string(b[36:40]) != "data" {
		t.Fatal("invalid WAV container")
	}
	if binary.LittleEndian.Uint32(b[4:]) != uint32(len(b)-8) || binary.LittleEndian.Uint32(b[40:]) != 220500 {
		t.Fatal("wrong chunk sizes")
	}
	if binary.LittleEndian.Uint16(b[20:]) != 1 || binary.LittleEndian.Uint16(b[22:]) != 1 || binary.LittleEndian.Uint32(b[24:]) != 22050 || binary.LittleEndian.Uint16(b[34:]) != 16 {
		t.Fatal("wrong PCM format")
	}
	peak := 0
	for i := 44; i < len(b); i += 2 {
		v := int(int16(binary.LittleEndian.Uint16(b[i:])))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	if peak < 3000 || peak > 3277 {
		t.Fatalf("unexpected volume %d", peak)
	}
	if binary.LittleEndian.Uint16(b[44:]) != 0 || binary.LittleEndian.Uint16(b[len(b)-2:]) != 0 {
		t.Fatal("missing endpoint fades")
	}
}
func TestRoutes(t *testing.T) {
	h, token, err := New()
	if err != nil {
		t.Fatal(err)
	}
	_, token2, err := New()
	if err != nil || token == token2 || len(token) != 48 {
		t.Fatal("per-run random token missing")
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 404}, {"GET", "/bad/", 404}, {"GET", "/" + token, 404}, {"GET", "/" + token + "/", 200}, {"GET", "/" + token + "/tone.wav", 200}, {"HEAD", "/" + token + "/tone.wav", 200}, {"POST", "/" + token + "/tone.wav", 405}, {"GET", "/" + token + "/other", 404}, {"GET", "/" + token + "/../tone.wav", 404},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unsafe headers")
			}
			if strings.Contains(w.Body.String(), token) {
				t.Fatal("token disclosed in response")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
		})
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/"+token+"/tone.wav", nil)
	r.Header.Set("Range", "bytes=0-43")
	h.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.Len() != 44 || w.Header().Get("Content-Range") != "bytes 0-43/220544" {
		t.Fatal("range unsupported", w.Code, w.Header())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/"+token+"/", nil))
	body, _ := io.ReadAll(w.Result().Body)
	if strings.Contains(string(body), "getUserMedia(") || strings.Contains(string(body), "https://") || strings.Contains(string(body), "http://") {
		t.Fatal("camera capture or remote resource introduced")
	}
	if !strings.Contains(string(body), "src=\"tone.wav\"") {
		t.Fatal("missing relative audio source")
	}
}

func TestSnapshot(t *testing.T) {
	h, token, _ := New()
	upload := func(data []byte, path string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		p, _ := mw.CreateFormFile("photo", "untrusted-name.jpg")
		p.Write(data)
		mw.Close()
		r := httptest.NewRequest("POST", path, &b)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 16, 16)), nil)
	if w := upload(b.Bytes(), "/bad/snapshot"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := upload([]byte("<svg/>"), "/"+token+"/snapshot"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := upload(bytes.Repeat([]byte{1}, (5<<20)+1), "/"+token+"/snapshot"); w.Code != 413 {
		t.Fatal(w.Code)
	}
	if w := upload(b.Bytes(), "/"+token+"/snapshot"); w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/"+token+"/snapshot.jpg", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	cfg, format, err := image.DecodeConfig(w.Body)
	if err != nil || format != "jpeg" || cfg.Width != 16 {
		t.Fatal("bad normalized image")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/"+token+"/clear", nil))
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/"+token+"/snapshot.jpg", nil))
	if w.Code != 404 {
		t.Fatal("not cleared")
	}
}
