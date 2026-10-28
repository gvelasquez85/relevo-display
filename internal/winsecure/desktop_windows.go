//go:build windows

// Package winsecure limits the privileged broker to the console sign-in desktop.
// It never switches the visible desktop and never permits the normal user GUI
// to enter a secure desktop.
package winsecure

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"
)

var enabled atomic.Bool
var user = windows.NewLazySystemDLL("user32.dll")
var kernel = windows.NewLazySystemDLL("kernel32.dll")

func Enabled() bool { return enabled.Load() }
func objectName(h uintptr) (string, error) {
	var name [256]uint16
	var needed uint32
	ok, _, err := user.NewProc("GetUserObjectInformationW").Call(h, 2, uintptr(unsafe.Pointer(&name[0])), unsafe.Sizeof(name), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return "", fmt.Errorf("desktop identity: %v", err)
	}
	return windows.UTF16ToString(name[:]), nil
}

// Enable requires SYSTEM in the active nonzero console session. The caller must
// additionally verify the installed executable path before invoking this mode.
func Enable() error {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if !u.User.Sid.IsWellKnown(windows.WinLocalSystemSid) {
		return fmt.Errorf("prelogin broker requires LocalSystem")
	}
	var session uint32
	if err = windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil {
		return err
	}
	console, _, _ := kernel.NewProc("WTSGetActiveConsoleSessionId").Call()
	if session == 0 || session != uint32(console) || uint32(console) == 0xffffffff {
		return fmt.Errorf("prelogin broker is outside active console")
	}
	station, _, _ := user.NewProc("GetProcessWindowStation").Call()
	name, err := objectName(station)
	if err != nil {
		return err
	}
	if !strings.EqualFold(name, "WinSta0") {
		return fmt.Errorf("prelogin broker is not interactive console")
	}
	enabled.Store(true)
	release, err := Enter()
	if err != nil {
		enabled.Store(false)
		return err
	}
	release()
	return nil
}

// Enter binds only the calling thread and restores it on release. Refusing the
// Default desktop prevents this SYSTEM process from controlling a signed-in user.
func Enter() (func(), error) {
	if !Enabled() {
		return func() {}, nil
	}
	runtime.LockOSThread()
	// Check again for every operation, not only the service's polling interval.
	// A locked signed-in user also has a Winlogon desktop; it is not pre-login.
	var session uint32
	console := windows.WTSGetActiveConsoleSessionId()
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil || console == 0xffffffff || session == 0 || session != console {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("prelogin console changed")
	}
	var token windows.Token
	tokenErr := windows.WTSQueryUserToken(session, &token)
	if tokenErr == nil {
		token.Close()
	}
	if !errors.Is(tokenErr, windows.ERROR_NO_TOKEN) {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("prelogin user state changed")
	}
	fail := func(err error) (func(), error) { runtime.UnlockOSThread(); return nil, err }
	tid, _, _ := kernel.NewProc("GetCurrentThreadId").Call()
	old, _, err := user.NewProc("GetThreadDesktop").Call(tid)
	if old == 0 {
		return fail(fmt.Errorf("GetThreadDesktop: %v", err))
	}
	input, _, err := user.NewProc("OpenInputDesktop").Call(0, 0, 0x181)
	if input == 0 {
		return fail(fmt.Errorf("OpenInputDesktop: %v", err))
	}
	name, err := objectName(input)
	if err != nil || !strings.EqualFold(name, "Winlogon") {
		user.NewProc("CloseDesktop").Call(input)
		return fail(fmt.Errorf("prelogin desktop is no longer Winlogon"))
	}
	ok, _, err := user.NewProc("SetThreadDesktop").Call(input)
	if ok == 0 {
		user.NewProc("CloseDesktop").Call(input)
		return fail(fmt.Errorf("SetThreadDesktop: %v", err))
	}
	return func() {
		ok, _, _ := user.NewProc("SetThreadDesktop").Call(old)
		if ok != 0 {
			user.NewProc("CloseDesktop").Call(input)
			runtime.UnlockOSThread()
		}
		// If restoration fails retain the locked thread: Go destroys it when the
		// calling goroutine exits rather than recycling a secure-desktop thread.
	}, nil
}
