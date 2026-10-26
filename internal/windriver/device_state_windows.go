//go:build windows

package windriver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var deviceCfg = syscall.NewLazyDLL("cfgmgr32.dll")

type stateNode struct {
	inst             uint32
	instance, driver string
	physical         bool
}

func stateProperty(h uintptr, d *devInfo, property uint32, wantType uint32) ([]uint16, error) {
	var data [4096]uint16
	var typ, size uint32
	ok, _, e := proc("SetupDiGetDeviceRegistryPropertyW").Call(h, uintptr(unsafe.Pointer(d)), uintptr(property), uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return nil, e
	}
	if typ != wantType || size < 2 || size > uint32(len(data)*2) || size%2 != 0 {
		return nil, fmt.Errorf("invalid display registry property %d", property)
	}
	return append([]uint16(nil), data[:size/2]...), nil
}
func stateHardwareIDs(data []uint16) []string {
	var ids []string
	for start := 0; start < len(data) && data[start] != 0; {
		end := start
		for end < len(data) && data[end] != 0 {
			end++
		}
		ids = append(ids, syscall.UTF16ToString(data[start:end]))
		start = end + 1
	}
	return ids
}
func stateNodes(ctx context.Context) ([]stateNode, []stateNode, error) {
	h, _, e := proc("SetupDiGetClassDevsW").Call(uintptr(unsafe.Pointer(&displayGUID)), 0, 0, 2)
	if h == ^uintptr(0) {
		return nil, nil, fmt.Errorf("enumerate present display devices: %v", e)
	}
	defer proc("SetupDiDestroyDeviceInfoList").Call(h)
	var targets, physical []stateNode
	for i := uintptr(0); i < 4096; i++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		d := devInfo{}
		d.Size = uint32(unsafe.Sizeof(d))
		ok, _, e := proc("SetupDiEnumDeviceInfo").Call(h, i, uintptr(unsafe.Pointer(&d)))
		if ok == 0 {
			if e == syscall.Errno(259) {
				return targets, physical, nil
			}
			return nil, nil, fmt.Errorf("enumerate display device: %v", e)
		}
		ids, e := stateProperty(h, &d, 1, 7)
		if e != nil {
			if e == syscall.Errno(13) || e == syscall.Errno(2) {
				continue
			}
			return nil, nil, e
		}
		target, isPhysical := false, false
		for _, id := range stateHardwareIDs(ids) {
			id = strings.ToLower(id)
			if id == `root\mttvdd` {
				target = true
			}
			if strings.HasPrefix(id, `pci\`) || strings.HasPrefix(id, `usb\`) || strings.HasPrefix(id, `acpi\`) {
				isPhysical = true
			}
		}
		if !target && !isPhysical {
			continue
		}
		driver, e := stateProperty(h, &d, 9, 1)
		if e != nil {
			return nil, nil, fmt.Errorf("read display driver identity: %v", e)
		}
		var instance [512]uint16
		r, _, _ := deviceCfg.NewProc("CM_Get_Device_IDW").Call(uintptr(d.Inst), uintptr(unsafe.Pointer(&instance[0])), uintptr(len(instance)), 0)
		if r != 0 {
			return nil, nil, fmt.Errorf("CM_Get_Device_IDW status %#x", r)
		}
		n := stateNode{inst: d.Inst, driver: syscall.UTF16ToString(driver), instance: syscall.UTF16ToString(instance[:]), physical: isPhysical && !target}
		if target {
			targets = append(targets, n)
		} else {
			physical = append(physical, n)
		}
	}
	return nil, nil, fmt.Errorf("display device enumeration safety limit exceeded")
}
func stateNodeStatus(n stateNode) (DeviceState, error) {
	var status, problem uint32
	r, _, _ := deviceCfg.NewProc("CM_Get_DevNode_Status").Call(uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&problem)), uintptr(n.inst), 0)
	if r != 0 {
		return DeviceState{}, fmt.Errorf("CM_Get_DevNode_Status status %#x", r)
	}
	return deviceState(n.instance, status, problem), nil
}
func ReadDeviceState(ctx context.Context) (DeviceState, error) {
	targets, _, e := stateNodes(ctx)
	if e != nil {
		return DeviceState{}, e
	}
	if len(targets) == 0 {
		return DeviceState{}, nil
	}
	if len(targets) != 1 {
		return DeviceState{}, fmt.Errorf("expected exactly one Root\\MttVDD adapter; found %d", len(targets))
	}
	return stateNodeStatus(targets[0])
}
func stateOwnedFiles() error {
	pinned, e := packageFiles(archive, ArchiveSHA256)
	if e != nil {
		return e
	}
	const root = `C:\VirtualDisplayDriver`
	st, e := os.Lstat(root)
	if e != nil {
		return e
	}
	attrs, e := syscall.GetFileAttributes(u16(root))
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || attrs&0x400 != 0 {
		return fmt.Errorf("unsafe driver package directory")
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return e
	}
	if len(entries) < len(pinned) || len(entries) > len(pinned)+1 {
		return fmt.Errorf("unexpected driver package contents")
	}
	actual := map[string][]byte{}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := pinned[name]; !ok && name != "vdd_settings.xml.yam-v03-backup" {
			return fmt.Errorf("unowned driver package entry %s", name)
		}
		path := filepath.Join(root, name)
		st, e := os.Lstat(path)
		if e != nil {
			return e
		}
		attrs, e := syscall.GetFileAttributes(u16(path))
		if e != nil || !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || attrs&0x400 != 0 || st.Size() > 4<<20 {
			return fmt.Errorf("unsafe driver package file %s", name)
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(f, (4<<20)+1))
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if len(b) > 4<<20 {
			return fmt.Errorf("oversized driver package file %s", name)
		}
		actual[name] = b
	}
	if e := validateOwnedFiles(actual, pinned); e != nil {
		return e
	}
	return validateSingleMonitorConfiguration(actual["vdd_settings.xml"])
}

// stateReadPackageFile follows no final reparse point and bounds every read.
func stateReadPackageFile(path string) ([]byte, error) {
	h, e := syscall.CreateFile(u16(path), syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, 0x00200000, 0)
	if e != nil {
		return nil, e
	}
	var info syscall.ByHandleFileInformation
	if e := syscall.GetFileInformationByHandle(h, &info); e != nil {
		syscall.CloseHandle(h)
		return nil, e
	}
	if info.FileAttributes&(0x400|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.FileSizeHigh != 0 || info.FileSizeLow > 4<<20 {
		syscall.CloseHandle(h)
		return nil, fmt.Errorf("unsafe installed driver file")
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("oversized installed driver file")
	}
	return data, nil
}
func stateCheckDirectoryChain(path, root string) error {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	relative, e := filepath.Rel(root, path)
	if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("driver package escaped Windows directory")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, e := os.Lstat(current)
		if e != nil {
			return e
		}
		attrs, e := syscall.GetFileAttributes(u16(current))
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || attrs&0x400 != 0 {
			return fmt.Errorf("unsafe installed driver directory")
		}
		if strings.EqualFold(current, root) {
			return nil
		}
		if filepath.Dir(current) == current {
			return fmt.Errorf("invalid installed driver directory ancestry")
		}
	}
}
func stateInstalledPackage(target stateNode) error {
	if !validDisplayDriverClassKey(target.driver) {
		return fmt.Errorf("invalid virtual display driver class key")
	}
	var h syscall.Handle
	if e := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, u16(`SYSTEM\CurrentControlSet\Control\Class\`+target.driver), 0, syscall.KEY_QUERY_VALUE, &h); e != nil {
		return e
	}
	var inf [512]uint16
	var typ uint32
	size := uint32(len(inf) * 2)
	e := syscall.RegQueryValueEx(h, u16("InfPath"), nil, &typ, (*byte)(unsafe.Pointer(&inf[0])), &size)
	syscall.RegCloseKey(h)
	if e != nil {
		return e
	}
	if typ != syscall.REG_SZ || size < 2 || size > uint32(len(inf)*2) || size%2 != 0 {
		return fmt.Errorf("invalid installed INF registry property")
	}
	published := syscall.UTF16ToString(inf[:size/2])
	if !validPublishedINF(published) {
		return fmt.Errorf("invalid published virtual display INF name")
	}
	var windows [32768]uint16
	n, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("GetWindowsDirectoryW").Call(uintptr(unsafe.Pointer(&windows[0])), uintptr(len(windows)))
	if n == 0 || n >= uintptr(len(windows)) {
		return fmt.Errorf("read Windows directory: %v", e)
	}
	windowsRoot := syscall.UTF16ToString(windows[:n])
	if !filepath.IsAbs(windowsRoot) {
		return fmt.Errorf("invalid Windows directory")
	}
	publishedPath := filepath.Join(windowsRoot, "INF", published)
	if e := stateCheckDirectoryChain(filepath.Dir(publishedPath), windowsRoot); e != nil {
		return e
	}
	publishedBytes, e := stateReadPackageFile(publishedPath)
	if e != nil {
		return e
	}
	pinned, e := packageFiles(archive, ArchiveSHA256)
	if e != nil {
		return e
	}
	if !bytes.Equal(publishedBytes, pinned["MttVDD.inf"]) {
		return fmt.Errorf("installed published INF differs from Relevo package")
	}
	var store [32768]uint16
	var required uint32
	ok, _, e := proc("SetupGetInfDriverStoreLocationW").Call(uintptr(unsafe.Pointer(u16(publishedPath))), 0, 0, uintptr(unsafe.Pointer(&store[0])), uintptr(len(store)), uintptr(unsafe.Pointer(&required)))
	if ok == 0 {
		return fmt.Errorf("resolve installed virtual driver store: %v", e)
	}
	if required == 0 || required > uint32(len(store)) {
		return fmt.Errorf("invalid driver store location size")
	}
	storeINF := syscall.UTF16ToString(store[:required])
	storeDir := filepath.Dir(storeINF)
	expectedRoot := filepath.Join(windowsRoot, "System32", "DriverStore", "FileRepository")
	rel, e := filepath.Rel(expectedRoot, storeDir)
	if e != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.ContainsAny(rel, `\/`) || !strings.EqualFold(filepath.Base(storeINF), "MttVDD.inf") {
		return fmt.Errorf("unexpected installed virtual driver store location")
	}
	if e := stateCheckDirectoryChain(storeDir, windowsRoot); e != nil {
		return e
	}
	actual := map[string][]byte{}
	for _, name := range []string{"MttVDD.inf", "MttVDD.dll", "mttvdd.cat"} {
		data, e := stateReadPackageFile(filepath.Join(storeDir, name))
		if e != nil {
			return e
		}
		actual[name] = data
	}

	if e := validateInstalledPackageFiles(actual, pinned); e != nil {
		return e
	}
	// The exact pinned INF binds its UMDF service to this installed binary.
	// Check that copy too, rather than trusting only the staged package.
	system, e := trustedSystemDirectory()
	if e != nil {
		return e
	}
	serviceDLL := filepath.Join(system, "drivers", "UMDF", "MttVDD.dll")
	if e := stateCheckDirectoryChain(filepath.Dir(serviceDLL), windowsRoot); e != nil {
		return e
	}
	serviceBytes, e := stateReadPackageFile(serviceDLL)
	if e != nil {
		return e
	}
	if !bytes.Equal(serviceBytes, pinned["MttVDD.dll"]) {
		return fmt.Errorf("installed UMDF service binary differs from Relevo package")
	}
	return nil
}

type stateDisplayDevice struct {
	Size        uint32
	Name        [32]uint16
	Description [128]uint16
	Flags       uint32
	ID, Key     [128]uint16
}

func stateDriverClassKey(path string) string {
	if direct := displayDriverClassKey(path); direct != "" {
		return direct
	}
	const root = `\Registry\Machine\`
	if !strings.HasPrefix(strings.ToLower(path), strings.ToLower(root)) {
		return ""
	}
	// EnumDisplayDevices exposes Control\Video registry symbolic links. Open
	// the link itself, read only its REG_LINK target, and compare the resolved
	// class-key identity with SetupAPI's independently obtained SPDRP_DRIVER.
	var h syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, u16(path[len(root):]), 8, syscall.KEY_QUERY_VALUE, &h) != nil {
		return ""
	}
	defer syscall.RegCloseKey(h)
	var data [4096]uint16
	var typ uint32
	size := uint32(len(data) * 2)
	if syscall.RegQueryValueEx(h, u16("SymbolicLinkValue"), nil, &typ, (*byte)(unsafe.Pointer(&data[0])), &size) != nil || typ != 6 || size == 0 || size%2 != 0 || size > uint32(len(data)*2) {
		return ""
	}
	return displayDriverClassKey(syscall.UTF16ToString(data[:size/2]))
}
func stateDisplayGuard(ctx context.Context, target stateNode, physical []stateNode) (deviceGuard, error) {
	g := deviceGuard{Count: 1, Owned: true}
	user := syscall.NewLazyDLL("user32.dll")
	wanted := strings.ToLower(target.driver)
	for i := uintptr(0); i < 4096; i++ {
		if e := ctx.Err(); e != nil {
			return g, e
		}
		d := stateDisplayDevice{}
		d.Size = uint32(unsafe.Sizeof(d))
		ok, _, _ := user.NewProc("EnumDisplayDevicesW").Call(0, i, uintptr(unsafe.Pointer(&d)), 0)
		if ok == 0 {
			return g, nil
		}
		if d.Flags&8 != 0 {
			continue
		}
		key := stateDriverClassKey(syscall.UTF16ToString(d.Key[:]))
		if key == wanted {
			g.SourceIdentified = true
			g.Primary = g.Primary || d.Flags&4 != 0
			g.Attached = g.Attached || d.Flags&1 != 0
			continue
		}
		if d.Flags&1 != 0 {
			for _, p := range physical {
				if key != "" && key == strings.ToLower(p.driver) {
					g.PhysicalAttached = true
				}
			}
		}
	}
	return g, fmt.Errorf("display source enumeration safety limit exceeded")
}

// SetDeviceEnabled changes only the verified pinned single-monitor adapter.
// Disable uses CM_DISABLE_UI_NOT_OK (0x4), not the similarly named HARDWARE
// flag (0x2), and never persists a disabled state across Windows reboots.
func SetDeviceEnabled(ctx context.Context, enabled bool, log func(string)) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := stateOwnedFiles(); e != nil {
		return fmt.Errorf("verify owned virtual adapter package: %w", e)
	}
	targets, physical, e := stateNodes(ctx)
	if e != nil {
		return e
	}
	if len(targets) != 1 {
		return validateDeviceTransition(deviceGuard{Count: len(targets)}, enabled)
	}
	target := targets[0]
	if e := stateInstalledPackage(target); e != nil {
		return fmt.Errorf("verify installed virtual display package: %w", e)
	}

	state, e := stateNodeStatus(target)
	if e != nil {
		return e
	}
	guard, e := stateDisplayGuard(ctx, target, physical)
	if e != nil {
		return e
	}
	// Disabled devices need not expose a GDI source at all. No native state
	// change is performed for an already disabled device.
	if state.Disabled && !enabled {
		guard.SourceIdentified = true
	}
	if e := validateDeviceTransition(guard, enabled); e != nil {
		return e
	}
	if !deviceNeedsChange(state, enabled) {
		return nil
	}
	admin, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("IsUserAnAdmin").Call()
	if admin == 0 {
		return ErrDeviceElevationRequired
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	op, flags := "CM_Disable_DevNode", uintptr(4)
	if enabled {
		op, flags = "CM_Enable_DevNode", 0
	}
	r, _, _ := deviceCfg.NewProc(op).Call(uintptr(target.inst), flags)
	if log != nil {
		log(fmt.Sprintf("virtual adapter %s flags=%#x status=%#x", op, flags, r))
	}
	if r != 0 {
		return &NativeDeviceError{Operation: op, Code: uint32(r)}
	}
	pollCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for {
		current, e := ReadDeviceState(pollCtx)
		if e != nil {
			return e
		}
		if current.Instance != target.instance {
			return fmt.Errorf("virtual adapter identity changed during state transition")
		}
		if enabled && current.Enabled || !enabled && current.Disabled {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-pollCtx.Done():
			timer.Stop()
			return fmt.Errorf("virtual adapter state was not confirmed (enabled=%t status=%#x problem=%d): %w", enabled, current.Status, current.Problem, pollCtx.Err())
		case <-timer.C:
		}
	}
}
