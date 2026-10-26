//go:build windows && amd64

package jpegfast

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed assets/libturbojpeg.dll
var dllImage []byte

const dllSHA256 = "4fd997c7eec636d3c48a9fc604b6cb573da030f2f6382d4ed07397c7d7f4e50e"

var native struct {
	mu          sync.RWMutex
	closed      bool
	shutdownErr error
	directory   string
	once        sync.Once
	err         error
	library     *syscall.DLL
	// Keep a read-only file handle open without write/delete sharing for the lifetime
	// of the process. The loaded DLL cannot be replaced after verification.
	locked                        *os.File
	init, compress, free, destroy *syscall.Proc
}

// EncodeBGRA encodes top-down BGRA directly with libjpeg-turbo SIMD. The caller
// must keep pixels unchanged until this synchronous call returns. No Go pointer
// is retained by the library. Each compressor and native output is freed.
func EncodeBGRA(pixels []byte, width, height, stride, quality int) ([]byte, error) {
	if err := validate(pixels, width, height, stride, quality); err != nil {
		return nil, err
	}
	native.mu.RLock()
	defer native.mu.RUnlock()
	if native.closed {
		return nil, fmt.Errorf("%w: encoder shut down", ErrUnavailable)
	}
	native.once.Do(loadNative)
	if native.err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, native.err)
	}
	return encodeNative(pixels, width, height, stride, quality)
}

// encodeNative is also used before publishing the initialized encoder, so the
// startup self-test never re-enters sync.Once or the lifecycle mutex.
func encodeNative(pixels []byte, width, height, stride, quality int) ([]byte, error) {
	handle, _, _ := native.init.Call()
	if handle == 0 {
		return nil, fmt.Errorf("TurboJPEG compressor initialization failed")
	}
	defer native.destroy.Call(handle)
	var output unsafe.Pointer
	// Windows unsigned long remains 32 bits on x64 (LLP64).
	var size uint32
	// TJPF_BGRA=8, TJSAMP_420=2, TJFLAG_FASTDCT=2048, verified
	// against the pinned release's include/turbojpeg.h. Windows x64 uses one
	// calling convention for C and Windows APIs; syscall.Proc.Call supplies it.
	rc, _, _ := native.compress.Call(handle, uintptr(unsafe.Pointer(&pixels[0])), uintptr(width), uintptr(stride), uintptr(height), 8, uintptr(unsafe.Pointer(&output)), uintptr(unsafe.Pointer(&size)), 2, uintptr(quality), 2048)
	runtime.KeepAlive(pixels)
	if output != nil {
		defer native.free.Call(uintptr(output))
	}
	if int32(rc) != 0 {
		return nil, fmt.Errorf("TurboJPEG compression failed")
	}
	if output == nil || size < 4 || size > 64*1024*1024 {
		return nil, fmt.Errorf("TurboJPEG returned invalid output size")
	}
	// One copy transfers ownership to Go; native allocation is then freed.
	result := bytes.Clone(unsafe.Slice((*byte)(output), int(size)))
	return result, nil
}

func noReparse(path string) error {
	// Reject junction/symlink ancestors as well as the destination itself.
	for {
		p, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attrs, err := syscall.GetFileAttributes(p)
		if err != nil {
			return err
		}
		if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("native encoder path contains reparse point")
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func loadNative() {
	fail := func(err error) { native.err = err }
	if fmt.Sprintf("%x", sha256.Sum256(dllImage)) != dllSHA256 {
		fail(fmt.Errorf("embedded encoder digest mismatch"))
		return
	}
	base, err := os.UserCacheDir()
	if err != nil {
		fail(err)
		return
	}
	if err = os.MkdirAll(base, 0700); err != nil {
		fail(err)
		return
	}
	if err = noReparse(base); err != nil {
		fail(err)
		return
	}
	// Unique directory avoids collisions with previous versions or processes.
	dir, err := os.MkdirTemp(base, "Relevo-JPEG-3.2.0-")
	if err != nil {
		fail(err)
		return
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(dir)
		}
	}()
	if err = noReparse(dir); err != nil {
		fail(err)
		return
	}
	path := filepath.Join(dir, "libturbojpeg-"+dllSHA256+".dll")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fail(err)
		return
	}
	_, err = f.Write(dllImage)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fail(err)
		return
	}
	if err = noReparse(path); err != nil {
		fail(err)
		return
	}
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		fail(err)
		return
	}
	h, err := syscall.CreateFile(wide, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		fail(err)
		return
	}
	locked := os.NewFile(uintptr(h), path)
	success := false
	defer func() {
		if !success {
			locked.Close()
		}
	}()
	digest := sha256.New()
	if _, err = io.Copy(digest, locked); err != nil {
		fail(err)
		return
	}
	if !strings.EqualFold(fmt.Sprintf("%x", digest.Sum(nil)), dllSHA256) {
		fail(fmt.Errorf("extracted encoder digest mismatch"))
		return
	}
	if err = noReparse(path); err != nil {
		fail(err)
		return
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	// Only the absolute DLL directory and Windows System32 may satisfy imports.
	libHandle, _, callErr := kernel.NewProc("LoadLibraryExW").Call(uintptr(unsafe.Pointer(wide)), 0, 0x100|0x800)
	runtime.KeepAlive(wide)
	if libHandle == 0 {
		fail(fmt.Errorf("LoadLibraryExW: %v", callErr))
		return
	}
	lib := &syscall.DLL{Name: path, Handle: syscall.Handle(libHandle)}
	names := []string{"tjInitCompress", "tjCompress2", "tjFree", "tjDestroy"}
	procs := make([]*syscall.Proc, len(names))
	for i, name := range names {
		procs[i], err = lib.FindProc(name)
		if err != nil {
			lib.Release()
			fail(err)
			return
		}
	}
	native.directory = dir
	native.library = lib
	native.locked = locked
	native.init, native.compress, native.free, native.destroy = procs[0], procs[1], procs[2], procs[3]
	if err := selfTest(); err != nil {
		lib.Release()
		native.library = nil
		native.directory = ""
		native.locked = nil
		fail(fmt.Errorf("native JPEG startup validation: %w", err))
		return
	}
	success = true
	cleanup = false
}

// Verify the loaded DLL and ABI on each process's first use before encoding any
// desktop pixels. A failed native self-test leaves the existing Go fallback.
func selfTest() error {
	pixels := make([]byte, 8*8*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i+2] = 255
		pixels[i+3] = 255
	}
	data, err := encodeNative(pixels, 8, 8, 8*4, 90)
	if err != nil {
		return err
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		return fmt.Errorf("unexpected self-test dimensions: %v", img.Bounds())
	}
	r, g, b, _ := img.At(4, 4).RGBA()
	if r < 60000 || g > 4000 || b > 4000 {
		return fmt.Errorf("incorrect BGRA color order: %d %d %d", r, g, b)
	}
	return nil
}

// Shutdown waits for encodes to finish, unloads the native library and removes
// its private extraction. Call once at final application exit, after capture
// workers stop. Later encodes return ErrUnavailable. It is safe to repeat and
// does not initialize an encoder that was never used.
func Shutdown() error {
	native.mu.Lock()
	defer native.mu.Unlock()
	if native.closed {
		return native.shutdownErr
	}
	native.closed = true
	if native.library == nil {
		return nil
	}
	if err := native.library.Release(); err != nil {
		native.shutdownErr = fmt.Errorf("unload JPEG encoder: %w", err)
		return native.shutdownErr
	}
	native.library = nil
	var cleanupErrors []error
	if native.locked != nil {
		if err := native.locked.Close(); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		native.locked = nil
	}
	if err := noReparse(native.directory); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	} else {
		// Remove precisely our file and empty directory, never recurse through or
		// erase unrecognized content if the private directory has been changed.
		path := filepath.Join(native.directory, "libturbojpeg-"+dllSHA256+".dll")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
		if err := os.Remove(native.directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	native.shutdownErr = errors.Join(cleanupErrors...)
	return native.shutdownErr
}
