//go:build linux

package desktop

import (
	"bufio"
	"context"
	"fmt"
	"github.com/godbus/dbus/v5"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// Linux capture uses the desktop's ScreenCast portal, including VIRTUAL for
// extension. It never substitutes an existing monitor for an unsupported virtual one.
func Open(ctx context.Context, opt Options) (Source, error) {
	width, height, err := opt.Dimensions()
	if err != nil {
		return nil, err
	}
	if opt.Mode == "extend" && os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		return openHyprland(ctx, opt)
	}
	if _, err := exec.LookPath("gst-launch-1.0"); err != nil {
		return nil, fmt.Errorf("Linux capture requires GStreamer gst-launch-1.0 with pipewire and jpeg plugins: %w", err)
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect desktop session bus: %w", err)
	}
	var session dbus.ObjectPath
	success := false
	defer func() {
		if !success {
			if session != "" {
				closePortalObject(conn, session, "org.freedesktop.portal.Session")
			}
			conn.Close()
		}
	}()
	propctx, propcancel := context.WithTimeout(ctx, 10*time.Second)
	defer propcancel()
	var prop dbus.Variant
	err = conn.Object(portalName, portalPath).CallWithContext(propctx, "org.freedesktop.DBus.Properties.Get", 0, portalIface, "AvailableSourceTypes").Store(&prop)
	if err != nil {
		return nil, err
	}
	available, ok := prop.Value().(uint32)
	if !ok {
		return nil, fmt.Errorf("invalid portal AvailableSourceTypes")
	}
	typ, err := portalSourceType(opt.Mode, available)
	if err != nil {
		return nil, err
	}
	result, err := portalRequest(ctx, conn, "CreateSession", map[string]dbus.Variant{"session_handle_token": dbus.MakeVariant(fmt.Sprintf("relevo_session_%d", time.Now().UnixNano()))})
	if err != nil {
		return nil, err
	}
	switch handle := result["session_handle"].Value().(type) {
	case string:
		session = dbus.ObjectPath(handle)
	case dbus.ObjectPath:
		session = handle
	default:
		return nil, fmt.Errorf("portal returned invalid session")
	}
	if !session.IsValid() {
		return nil, fmt.Errorf("portal returned invalid session path")
	}
	if _, err = portalRequest(ctx, conn, "SelectSources", session, map[string]dbus.Variant{"types": dbus.MakeVariant(typ), "multiple": dbus.MakeVariant(false)}); err != nil {
		return nil, err
	}
	if opt.Log != nil {
		opt.Log("Selecciona o autoriza la pantalla en el diálogo del sistema.")
	}
	result, err = portalRequest(ctx, conn, "Start", session, "", map[string]dbus.Variant{})
	if err != nil {
		return nil, err
	}
	node, err := portalNode(result["streams"])
	if err != nil {
		return nil, err
	}
	callctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var fd dbus.UnixFD
	if err = conn.Object(portalName, portalPath).CallWithContext(callctx, portalIface+".OpenPipeWireRemote", 0, session, map[string]dbus.Variant{}).Store(&fd); err != nil {
		return nil, err
	}
	remote := os.NewFile(uintptr(fd), "portal-pipewire")
	if remote == nil {
		return nil, fmt.Errorf("invalid PipeWire descriptor")
	}
	defer remote.Close()
	fps := opt.FPS
	if fps < 1 || fps > 60 {
		fps = 15
	}
	runctx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(runctx, "gst-launch-1.0", portalGstArgsSize(node, fps, width, height)...)
	cmd.ExtraFiles = []*os.File{remote}
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stop()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		stop()
		stdout.Close()
		return nil, err
	}
	s := &linuxSource{frames: make(chan []byte, 1), done: make(chan struct{}), stop: stop, conn: conn, session: session, cmd: cmd}
	success = true
	go s.read(stdout)
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-s.done:
		}
	}()
	return s, nil
}
func portalGstArgs(node uint32, fps int) []string {
	return portalGstArgsSize(node, fps, Width, Height)
}
func portalGstArgsSize(node uint32, fps, width, height int) []string {
	return []string{"-q", "pipewiresrc", "fd=3", "path=" + strconv.FormatUint(uint64(node), 10), "!", "videoconvert", "!", "videoscale", "!", "videorate", "!", fmt.Sprintf("video/x-raw,width=%d,height=%d,pixel-aspect-ratio=1/1,framerate=%d/1", width, height, fps), "!", "jpegenc", "!", "fdsink", "fd=1"}
}

type linuxSource struct {
	frames  chan []byte
	done    chan struct{}
	stop    context.CancelFunc
	conn    *dbus.Conn
	session dbus.ObjectPath
	cmd     *exec.Cmd
	once    sync.Once
	err     error
}

func (s *linuxSource) read(r io.ReadCloser) {
	defer close(s.done)
	defer r.Close()
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		frame, err := ReadJPEG(reader)
		if err != nil {
			s.err = err
			break
		}
		select {
		case s.frames <- frame:
		default:
			select {
			case <-s.frames:
			default:
			}
			select {
			case s.frames <- frame:
			default:
			}
		}
	}
	s.stop() // Unblock Wait if the stream ended while the child is still alive.
	if err := s.cmd.Wait(); err != nil {
		s.err = fmt.Errorf("GStreamer capture stopped (verify pipewire/jpeg plugins): %w", err)
	}
}
func (s *linuxSource) Next(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, s.err
	case frame := <-s.frames:
		return frame, nil
	}
}
func (s *linuxSource) Close() error {
	s.once.Do(func() {
		s.stop()
		<-s.done
		closePortalObject(s.conn, s.session, "org.freedesktop.portal.Session")
		s.conn.Close()
	})
	return nil
}
