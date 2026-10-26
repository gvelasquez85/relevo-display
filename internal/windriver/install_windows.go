//go:build windows

package windriver

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed assets/VirtualDisplayDriver-x86.Driver.Only.zip
var archive []byte

var setup = syscall.NewLazyDLL("setupapi.dll")
var displayGUID = guid{0x4d36e968, 0xe325, 0x11ce, [8]byte{0xbf, 0xc1, 0x08, 0, 0x2b, 0xe1, 0x03, 0x18}}

type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}
type devInfo struct {
	Size     uint32
	Class    guid
	Inst     uint32
	Reserved uintptr
}

func u16(s string) *uint16               { return syscall.StringToUTF16Ptr(s) }
func proc(name string) *syscall.LazyProc { return setup.NewProc(name) }

func trustedSystemDirectory() (string, error) {
	var buf [32768]uint16
	n, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || n >= uintptr(len(buf)) {
		return "", fmt.Errorf("query Windows system directory: %v", e)
	}
	path := syscall.UTF16ToString(buf[:n])
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("invalid Windows system directory")
	}
	return path, nil
}
func Supported() bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	// SYSTEM_INFO begins with the native processor architecture WORD.
	var info [8]uintptr
	syscall.NewLazyDLL("kernel32.dll").NewProc("GetNativeSystemInfo").Call(uintptr(unsafe.Pointer(&info[0])))
	return *(*uint16)(unsafe.Pointer(&info[0])) == 9 // PROCESSOR_ARCHITECTURE_AMD64
}

func existing() (bool, error) {
	h, _, e := proc("SetupDiGetClassDevsW").Call(uintptr(unsafe.Pointer(&displayGUID)), 0, 0, 0)
	if h == ^uintptr(0) {
		return false, fmt.Errorf("enumerate display drivers: %w", e)
	}
	defer proc("SetupDiDestroyDeviceInfoList").Call(h)
	for i := uintptr(0); ; i++ {
		d := devInfo{}
		d.Size = uint32(unsafe.Sizeof(d))
		ok, _, e := proc("SetupDiEnumDeviceInfo").Call(h, i, uintptr(unsafe.Pointer(&d)))
		if ok == 0 {
			if e == syscall.Errno(259) {
				return false, nil
			}
			return false, fmt.Errorf("enumerate display device: %w", e)
		}
		var b [4096]uint16
		ok, _, e = proc("SetupDiGetDeviceRegistryPropertyW").Call(h, uintptr(unsafe.Pointer(&d)), 1, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)*2), 0)
		if ok == 0 {
			if e == syscall.Errno(13) || e == syscall.Errno(2) {
				continue
			}
			return false, fmt.Errorf("read display hardware IDs: %w", e)
		}
		for start := 0; start < len(b) && b[start] != 0; {
			end := start
			for end < len(b) && b[end] != 0 {
				end++
			}
			if strings.EqualFold(syscall.UTF16ToString(b[start:end]), `Root\MttVDD`) {
				return true, nil
			}
			start = end + 1
		}
	}
}

func Install(ctx context.Context, log func(string)) error {
	if log == nil {
		log = func(string) {}
	}
	if !Supported() {
		return fmt.Errorf("bundled virtual display driver supports Windows amd64 only")
	}
	admin, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("IsUserAnAdmin").Call()
	if admin == 0 {
		return fmt.Errorf("driver installation requires an elevated administrator token; run this executable as administrator and retry --install-driver")
	}
	found, e := existing()
	if e != nil {
		return e
	}
	if found {
		return prepareIPadConfig(log)
	}
	files, e := packageFiles(archive, ArchiveSHA256)
	if e != nil {
		return e
	}
	root := `C:\VirtualDisplayDriver`
	info, e := os.Lstat(root)
	if e == nil {
		attrs, attrErr := syscall.GetFileAttributes(u16(root))
		if attrErr != nil {
			return attrErr
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || attrs&0x400 != 0 {
			return fmt.Errorf("unsafe existing driver directory %s", root)
		}
		entries, e := os.ReadDir(root)
		if e != nil {
			return e
		}

		if len(entries) < len(files) || len(entries) > len(files)+1 {
			return fmt.Errorf("existing %s differs from owned package; refusing overwrite", root)
		}
		actual := make(map[string][]byte, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if _, known := files[name]; !known && name != "vdd_settings.xml.yam-v03-backup" {
				return fmt.Errorf("unowned driver directory entry %s", name)
			}
			p := filepath.Join(root, name)
			st, e := os.Lstat(p)
			attrs, attrErr := syscall.GetFileAttributes(u16(p))
			if e != nil || attrErr != nil || !st.Mode().IsRegular() || attrs&0x400 != 0 {
				return fmt.Errorf("unsafe existing driver file %s", p)
			}
			if st.Size() > 4<<20 {
				return fmt.Errorf("oversized existing driver file %s", p)
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			actual[name] = b
		}
		if e = validateOwnedFiles(actual, files); e != nil {
			return fmt.Errorf("existing %s differs from owned package: %w", root, e)
		}

	} else if os.IsNotExist(e) {
		// CreateDirectory is exclusive: a preexisting directory is never overwritten.
		if e = os.Mkdir(root, 0755); e != nil {
			return e
		}
		for name, b := range files {
			f, e := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if e != nil {
				return e
			}
			_, e = f.Write(b)
			ce := f.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
		}
	} else {
		return e
	}
	if e = prepareIPadConfig(log); e != nil {
		return e
	}
	// Authenticode uses the normal Windows trust policy, without importing certificates.
	systemDir, e := trustedSystemDirectory()
	if e != nil {
		return e
	}
	ps := filepath.Join(systemDir, "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", `$s = Get-AuthenticodeSignature -LiteralPath 'C:\VirtualDisplayDriver\mttvdd.cat'; if ($s.Status -ne 'Valid') { Write-Error ('Driver catalog trust verification failed: ' + $s.Status); exit 1 }`)
	if out, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("Windows did not trust driver catalog: %w: %s", e, strings.TrimSpace(string(out)))
	}
	inf := filepath.Join(root, "MttVDD.inf")
	cmd = exec.CommandContext(ctx, filepath.Join(systemDir, "pnputil.exe"), "/add-driver", inf, "/install")
	out, e := cmd.CombinedOutput()
	log(strings.TrimSpace(string(out)))
	if e != nil {
		if x, ok := e.(*exec.ExitError); ok && x.ExitCode() == 3010 {
			log("Windows requests a reboot.")
		} else {
			return fmt.Errorf("stage signed driver: %w", e)
		}
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	found, e = existing()
	if e != nil {
		return e
	}
	if found {
		log("Virtual display was installed concurrently; reusing it.")
		return nil
	}
	h, _, e := proc("SetupDiCreateDeviceInfoList").Call(uintptr(unsafe.Pointer(&displayGUID)), 0)
	if h == ^uintptr(0) {
		return fmt.Errorf("create display device list: %w", e)
	}
	defer proc("SetupDiDestroyDeviceInfoList").Call(h)
	d := devInfo{}
	d.Size = uint32(unsafe.Sizeof(d))
	ok, _, e := proc("SetupDiCreateDeviceInfoW").Call(h, uintptr(unsafe.Pointer(u16("MttVDD"))), uintptr(unsafe.Pointer(&displayGUID)), 0, 0, 1, uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		return fmt.Errorf("create virtual display node: %w", e)
	}
	ids := syscall.StringToUTF16(`Root\MttVDD`)
	ids = append(ids, 0)
	ok, _, e = proc("SetupDiSetDeviceRegistryPropertyW").Call(h, uintptr(unsafe.Pointer(&d)), 1, uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)*2))
	if ok == 0 {
		return fmt.Errorf("set virtual display hardware ID: %w", e)
	}
	ok, _, e = proc("SetupDiCallClassInstaller").Call(0x19, h, uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		return fmt.Errorf("register virtual display: %w", e)
	}
	var reboot int32
	ok, _, e = syscall.NewLazyDLL("newdev.dll").NewProc("UpdateDriverForPlugAndPlayDevicesW").Call(0, uintptr(unsafe.Pointer(u16(`Root\MttVDD`))), uintptr(unsafe.Pointer(u16(inf))), 0, uintptr(unsafe.Pointer(&reboot)))
	if ok == 0 {
		removed, _, removeErr := proc("SetupDiCallClassInstaller").Call(5, h, uintptr(unsafe.Pointer(&d)))
		if removed == 0 {
			return fmt.Errorf("bind virtual display driver: %w; cleanup of newly created node failed: %v", e, removeErr)
		}
		return fmt.Errorf("bind virtual display driver: %w (newly created node removed)", e)
	}
	if reboot != 0 {
		log("Virtual display installed; Windows requests a reboot.")
	} else {
		log("Virtual display installed. Windows display settings can now extend the desktop.")
	}
	return nil
}

// prepareIPadConfig migrates only our exact pinned v0.3 configuration. Unknown
// third-party settings are never overwritten, and the original is backed up.
func prepareIPadConfig(log func(string)) error {
	files, e := packageFiles(archive, ArchiveSHA256)
	if e != nil {
		return e
	}
	original := files["vdd_settings.xml"]
	desired, e := ipadConfiguration(original)
	if e != nil {
		return e
	}
	root := `C:\VirtualDisplayDriver`
	path := filepath.Join(root, "vdd_settings.xml")
	for _, p := range []string{root, path} {
		st, e := os.Lstat(p)
		if e != nil {
			return e
		}
		attrs, e := syscall.GetFileAttributes(u16(p))
		if e != nil || st.Mode()&os.ModeSymlink != 0 || attrs&0x400 != 0 {
			return fmt.Errorf("unsafe driver configuration path %s", p)
		}
	}
	current, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if bytes.Equal(current, desired) {
		log("iPad 4:3 resolutions already configured.")
		return nil
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("existing driver configuration is customized; left unchanged. Add 1024x768, 1280x960, 1600x1200 and 2048x1536 at 60 Hz to %s, then restart Windows", path)
	}
	backup := path + ".yam-v03-backup"
	f, e := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		if !os.IsExist(e) {
			return e
		}
		st, se := os.Lstat(backup)
		attrs, ae := syscall.GetFileAttributes(u16(backup))
		if se != nil || ae != nil || !st.Mode().IsRegular() || attrs&0x400 != 0 {
			return fmt.Errorf("unsafe configuration backup")
		}
		b, re := os.ReadFile(backup)
		if re != nil || !bytes.Equal(b, original) {
			return fmt.Errorf("configuration backup differs; refusing overwrite")
		}
	} else {
		_, e = f.Write(original)
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	// Save a new file first; rename with MoveFileEx replaces the validated config.
	temp := path + ".yam-new"
	f, e = os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		return e
	}
	defer os.Remove(temp)
	_, e = f.Write(desired)
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	ok, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(u16(temp))), uintptr(unsafe.Pointer(u16(path))), 1|8)
	if ok == 0 {
		return fmt.Errorf("activate iPad configuration: %v", e)
	}
	log("iPad 4:3 modes added; original saved as vdd_settings.xml.yam-v03-backup. Restart Windows to make the driver advertise the new modes.")
	return nil
}
