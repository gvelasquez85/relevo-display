package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"relevo/internal/jpegfast"
	"relevo/internal/windriver"
	"relevo/internal/winsecure"
	"relevo/internal/winvirtual"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var gdi32 = syscall.NewLazyDLL("gdi32.dll")
var getDC = user32.NewProc("GetDC")
var releaseDC = user32.NewProc("ReleaseDC")
var enumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
var getMonitorInfo = user32.NewProc("GetMonitorInfoW")
var createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
var createDIBSection = gdi32.NewProc("CreateDIBSection")
var bitBlt = gdi32.NewProc("BitBlt")
var gdiFlush = gdi32.NewProc("GdiFlush")
var selectObject = gdi32.NewProc("SelectObject")
var deleteObject = gdi32.NewProc("DeleteObject")
var deleteDC = gdi32.NewProc("DeleteDC")
var stretchBlt = gdi32.NewProc("StretchBlt")
var patBlt = gdi32.NewProc("PatBlt")

type winRect struct{ Left, Top, Right, Bottom int32 }
type monitorInfo struct {
	Size          uint32
	Monitor, Work winRect
	Flags         uint32
}
type bitmapInfo struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	ClrUsed, ClrImportant  uint32
	Colors                 [1]uint32
}
type winSource struct {
	recovery       captureRecovery
	topologyUntil  time.Time
	primary        bool
	status         CaptureStatus
	rect           winRect
	width, height  int
	paused         []byte
	mu             sync.Mutex
	closed         bool
	lease          *winvirtual.Lease
	backend        windowsDisplayBackend
	mutex          syscall.Handle
	closeErr       error
	quality        int
	encoder        string
	log            func(string)
	nativeLogged   bool
	fallbackLogged bool
	mem, bmp       uintptr
	pixels         []byte
	rgba           []byte
}

func Open(ctx context.Context, opt Options) (source Source, result error) {
	release, bindingErr := winsecure.Enter()
	if bindingErr != nil {
		return nil, bindingErr
	}
	defer release()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	width, height, err := opt.Dimensions()
	if err != nil {
		return nil, err
	}
	quality, err := opt.Quality()
	if err != nil {
		return nil, err
	}
	var pause bytes.Buffer
	if err := jpeg.Encode(&pause, image.NewRGBA(image.Rect(0, 0, width, height)), &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	if opt.Mode == "extend" {
		var handle syscall.Handle
		handle, err = acquireVirtualMutex()
		if err != nil {
			return nil, err
		}
		ready := false
		defer func() {
			if !ready {
				// A canceled start can finish enabling PnP before cancellation
				// is observed. Complete its inverse before releasing ownership.
				var cleanupErr error
				if !winsecure.Enabled() {
					cleanupErr = disableVirtualMonitorSafely(context.Background(), opt.Log)
				}
				result = errors.Join(result, cleanupErr, syscall.CloseHandle(handle))
			}
		}()
		if err = windriver.EnsureDeviceEnabled(ctx, true, opt.Log); err != nil {
			return nil, err
		}
		b := windowsDisplayBackend{log: opt.Log}
		var mode winvirtual.Mode
		// PnP may report STARTED before its display source is enumerated.
		startDeadline := time.Now().Add(5 * time.Second)
		for {
			mode, err = virtualCaptureMode(ctx, b, width, height)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if time.Now().After(startDeadline) {
				return nil, err
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		var lease *winvirtual.Lease
		lease, err = winvirtual.AcquirePosition(ctx, b, mode.Width, mode.Height, opt.Position)
		if err != nil {
			return nil, err
		}
		ready = true
		return &winSource{lease: lease, backend: b, mutex: handle, width: width, height: height, paused: pause.Bytes(), quality: quality, log: opt.Log}, nil
	}
	if opt.Mode != "desktop" {
		return nil, fmt.Errorf("unsupported Windows capture mode %q", opt.Mode)
	}
	if opt.Display < 1 {
		return nil, fmt.Errorf("display must be at least 1")
	}
	// Establish physical-pixel coordinates where the caller's DPI context permits it.
	if p := user32.NewProc("SetThreadDpiAwarenessContext"); p.Find() == nil {
		old, _, _ := p.Call(^uintptr(3))
		if old != 0 {
			defer p.Call(old)
		}
	}
	var primary []winRect
	var others []winRect
	cb := syscall.NewCallback(func(m, dc, r, data uintptr) uintptr {
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		ok, _, _ := getMonitorInfo.Call(m, uintptr(unsafe.Pointer(&info)))
		if ok != 0 {
			if info.Flags&1 != 0 {
				primary = append(primary, info.Monitor)
			} else {
				others = append(others, info.Monitor)
			}
		}
		return 1
	})
	ok, _, err := enumDisplayMonitors.Call(0, 0, cb, 0)
	if ok == 0 {
		return nil, fmt.Errorf("enumerate monitors: %v", err)
	}
	monitors := append(primary, others...)
	if opt.Display > len(monitors) {
		return nil, fmt.Errorf("display %d unavailable (%d monitors)", opt.Display, len(monitors))
	}
	return &winSource{rect: monitors[opt.Display-1], width: width, height: height, paused: pause.Bytes(), quality: quality, log: opt.Log}, nil
}
func (s *winSource) Next(ctx context.Context) ([]byte, error) {
	release, bindingErr := winsecure.Enter()
	if bindingErr != nil {
		return nil, bindingErr
	}
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if p := user32.NewProc("SetThreadDpiAwarenessContext"); p.Find() == nil {
		old, _, _ := p.Call(^uintptr(3))
		if old != 0 {
			defer p.Call(old)
		}
	}
	if s.closed {
		return nil, fmt.Errorf("desktop capture is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	unavailable := false
	available := func() (bool, error) {
		ok, e := captureDesktopAvailable()
		if e == nil && !ok {
			unavailable = true
		}
		return ok, e
	}
	frame, err := guardedCapture(ctx, available, func() ([]byte, error) { return s.captureFrame(ctx) }, s.paused)
	transient := err != nil
	frame, err = s.recovery.result(time.Now(), frame, err, s.paused)
	if err == nil {
		s.status.observe(unavailable || transient)
	}
	return frame, err
}

func (s *winSource) captureFrame(ctx context.Context) (frame []byte, resultErr error) {
	width, height := s.width, s.height
	if s.lease != nil {
		d, err := captureDisplay(ctx, s.lease.DisplayName)
		if err != nil {
			return nil, err
		}
		if err := validateLiveCaptureMode(d.Mode); err != nil {
			return nil, displayTransitionError{err}
		}
		liveRect := winRect{d.Mode.X, d.Mode.Y, d.Mode.X + d.Mode.Width, d.Mode.Y + d.Mode.Height}
		if liveRect != s.rect || s.primary != d.Primary {
			s.releaseCaptureResources()
			s.topologyUntil = time.Now().Add(10 * time.Second)
			s.rect = liveRect
			s.primary = d.Primary
			if s.log != nil {
				s.log("virtual display topology changed; following live geometry and primary status")
			}
		}
	}
	defer func() {
		if resultErr != nil {
			s.releaseCaptureResources()
		}
		if resultErr != nil && ctx.Err() == nil {
			if time.Now().Before(s.topologyUntil) {
				resultErr = displayTransitionError{resultErr}
			} else {
				resultErr = temporaryCaptureError{resultErr}
			}
		}
	}()
	dc, _, err := getDC.Call(0)
	if dc == 0 {
		return nil, fmt.Errorf("GetDC: %v", err)
	}
	defer releaseDC.Call(0, dc)
	if s.mem == 0 {
		mem, _, e := createCompatibleDC.Call(dc)
		if mem == 0 {
			return nil, fmt.Errorf("CreateCompatibleDC: %v", e)
		}
		bi := bitmapInfo{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32, SizeImage: uint32(width * height * 4)}
		var bits unsafe.Pointer
		bmp, _, e := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == nil {
			if bmp != 0 {
				deleteObject.Call(bmp)
			}
			deleteDC.Call(mem)
			return nil, fmt.Errorf("CreateDIBSection: %v", e)
		}
		s.mem, s.bmp = mem, bmp
		// GDI owns this memory until DeleteObject. Access stays under s.mu and
		// follows GdiFlush, so drawing and CPU reads cannot race.
		s.pixels = unsafe.Slice((*byte)(bits), width*height*4)
		s.rgba = make([]byte, width*height*4)
	}
	mem, bmp := s.mem, s.bmp
	old, _, err := selectObject.Call(mem, bmp)
	if old == 0 || old == ^uintptr(0) {
		return nil, fmt.Errorf("SelectObject: %v", err)
	}
	r := s.rect
	dest := winFitRectSize(r.Right-r.Left, r.Bottom-r.Top, int32(width), int32(height))
	if dest.Left != 0 || dest.Top != 0 || dest.Right != int32(width) || dest.Bottom != int32(height) {
		clearOK, _, clearErr := patBlt.Call(mem, 0, 0, uintptr(width), uintptr(height), 0x42)
		if clearOK == 0 {
			selectObject.Call(mem, old)
			return nil, fmt.Errorf("PatBlt: %v", clearErr)
		}
	}
	var ok uintptr
	var bltErr error
	if r.Right-r.Left == int32(width) && r.Bottom-r.Top == int32(height) {
		ok, _, bltErr = bitBlt.Call(mem, 0, 0, uintptr(width), uintptr(height), dc, uintptr(int64(r.Left)), uintptr(int64(r.Top)), 0x40CC0020)
	} else {
		ok, _, bltErr = stretchBlt.Call(mem, uintptr(dest.Left), uintptr(dest.Top), uintptr(dest.Right-dest.Left), uintptr(dest.Bottom-dest.Top), dc, uintptr(int64(r.Left)), uintptr(int64(r.Top)), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x40CC0020)
	}
	var cursorErr error
	if ok != 0 {
		cursorErr = drawCaptureCursor(mem, r, dest)
	}
	restored, _, restoreErr := selectObject.Call(mem, old)
	if restored == 0 || restored == ^uintptr(0) {
		return nil, fmt.Errorf("restore capture bitmap: %v", restoreErr)
	}
	if ok == 0 {
		return nil, fmt.Errorf("capture blit: %v", bltErr)
	}
	if cursorErr != nil {
		return nil, cursorErr
	}
	flushed, _, flushErr := gdiFlush.Call()
	if flushed == 0 {
		return nil, fmt.Errorf("GdiFlush: %v", flushErr)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if encoded, err := jpegfast.EncodeBGRA(s.pixels, width, height, width*4, s.quality); err == nil {
		s.reportEncoder("JPEG SIMD", nil)
		return encoded, nil
	} else {
		s.reportEncoder("JPEG Go", err)
	}
	pixels := s.rgba
	bgraToRGBA(pixels, s.pixels)
	return EncodeJPEGQuality(&image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}, width, height, s.quality)
}

// EncoderName reports the implementation that produced the latest frame.
func (s *winSource) EncoderName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoder
}

// Called under s.mu; report native success and fallback at most once each.
func (s *winSource) reportEncoder(name string, fallback error) {
	s.encoder = name
	logged := s.nativeLogged
	if fallback != nil {
		logged = s.fallbackLogged
	}
	if s.log != nil && !logged {
		message := "Capture encoder: " + name
		if fallback != nil {
			message += " (native fallback: " + fallback.Error() + ")"
		}
		s.log(message)
		if fallback != nil {
			s.fallbackLogged = true
		} else {
			s.nativeLogged = true
		}
	}
}

func (s *winSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	s.releaseCaptureResources()
	if s.lease != nil && !winsecure.Enabled() {
		// Retire the dedicated PnP adapter, not only its desktop path.
		// Camera, audio, microphone and input devices use separate adapters.
		s.closeErr = disableVirtualMonitorSafely(context.Background(), s.log)
	}
	if s.mutex != 0 {
		s.closeErr = errors.Join(s.closeErr, syscall.CloseHandle(s.mutex))
		s.mutex = 0
	}
	return s.closeErr
}

// releaseCaptureResources runs under s.mu, after every transient capture failure.
// A resumed desktop therefore gets GDI objects compatible with its current device.
func (s *winSource) releaseCaptureResources() {
	if s.mem != 0 {
		deleteDC.Call(s.mem)
		s.mem = 0
	}
	if s.bmp != 0 {
		deleteObject.Call(s.bmp)
		s.bmp = 0
	}
	s.pixels = nil
	s.rgba = nil
}

// winFitRect centers a monitor within the negotiated frame without distortion.
func winFitRect(width, height int32) winRect { return winFitRectSize(width, height, Width, Height) }
func winFitRectSize(width, height, outputWidth, outputHeight int32) winRect {
	if width <= 0 || height <= 0 {
		return winRect{}
	}
	w, h := int64(outputWidth), int64(outputHeight)
	if int64(width)*h > int64(height)*w {
		h = int64(height) * w / int64(width)
	} else {
		w = int64(width) * h / int64(height)
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	x, y := (int64(outputWidth)-w)/2, (int64(outputHeight)-h)/2
	return winRect{int32(x), int32(y), int32(x + w), int32(y + h)}
}

func (s *winSource) CaptureStatus() CaptureStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}
