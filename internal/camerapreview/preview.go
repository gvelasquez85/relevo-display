// Package camerapreview provides an explicitly local, RAM-only preview, not a virtual camera.
package camerapreview

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	mu       sync.RWMutex
	latest   []byte
	server   *http.Server
	listener net.Listener
	token    string
	viewers  chan struct{}
}

func New() (*Server, error) {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	s := &Server{listener: l, token: hex.EncodeToString(b), viewers: make(chan struct{}, 2)}
	s.server = &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 3 * time.Second}
	go s.server.Serve(l)
	return s, nil
}
func (s *Server) URL() string    { return "http://" + s.listener.Addr().String() + "/" + s.token + "/" }
func (s *Server) OBSURL() string { return s.URL() + "obs" }
func (s *Server) Close() error   { return s.server.Close() }
func (s *Server) Update(b []byte) error {
	if len(b) > 512*1024 {
		return fmt.Errorf("camera JPEG too large")
	}
	c, e := jpeg.DecodeConfig(bytes.NewReader(b))
	if e != nil || c.Width < 1 || c.Height < 1 || c.Width > 640 || c.Height > 480 {
		return fmt.Errorf("invalid camera JPEG")
	}
	s.mu.Lock()
	s.latest = append(s.latest[:0], b...)
	s.mu.Unlock()
	return nil
}
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/"+s.token+"/" && r.URL.Path != "/"+s.token+"/stream" && r.URL.Path != "/"+s.token+"/obs" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.URL.Path == "/"+s.token+"/obs" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><style>html,body{margin:0;width:640px;height:480px;overflow:hidden;background:black}img{width:640px;height:480px;object-fit:contain}</style><img src="stream">`)
		return
	}
	if r.URL.Path == "/"+s.token+"/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Relevo camera preview</title><h1>Relevo — local camera preview</h1><p>This preview is not a Windows camera device.</p><img src="stream" alt="iPad camera">`)
		return
	}
	select {
	case s.viewers <- struct{}{}:
		defer func() { <-s.viewers }()
	default:
		http.Error(w, "preview viewer limit", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=relevo")
	f, ok := w.(http.Flusher)
	if !ok {
		return
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			s.mu.RLock()
			b := append([]byte(nil), s.latest...)
			s.mu.RUnlock()
			if len(b) == 0 {
				continue
			}
			http.NewResponseController(w).SetWriteDeadline(time.Now().Add(3 * time.Second))
			if _, e := fmt.Fprintf(w, "--relevo\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(b)); e != nil {
				return
			}
			if _, e := w.Write(b); e != nil {
				return
			}
			if _, e := w.Write([]byte("\r\n")); e != nil {
				return
			}
			f.Flush()
		}
	}
}
