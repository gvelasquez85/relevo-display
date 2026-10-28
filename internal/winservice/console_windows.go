//go:build windows

package winservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"relevo/internal/wininstall"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const consoleHandoffEvent = `Global\RelevoDisplayConsoleHandoff`

const preloginStopEvent = `Global\RelevoDisplayPreloginStop`

func installedExecutable() (string, error) {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Relevo Display", "Relevo-Windows.exe"), nil
}
func validateInstalledExecutable(executable string) error {
	expected, err := installedExecutable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(executable), expected) {
		return errors.New("el servicio requiere el ejecutable instalado en Program Files\\Relevo Display")
	}
	// A normal user must not redirect a privileged child through a junction.
	for path := expected; ; path = filepath.Dir(path) {
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attr, err := windows.GetFileAttributes(p)
		if err != nil {
			return err
		}
		if attr&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("ruta protegida del servicio redirigida")
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
	}
	st, err := os.Stat(expected)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return errors.New("ejecutable de servicio inválido")
	}
	return wininstall.VerifyProtectedExecutable(executable)
}
func systemToken(access uint32) (windows.Token, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), access|windows.TOKEN_QUERY, &token); err != nil {
		return 0, err
	}
	user, err := token.GetTokenUser()
	if err != nil {
		token.Close()
		return 0, err
	}
	if !user.User.Sid.IsWellKnown(windows.WinLocalSystemSid) {
		token.Close()
		return 0, errors.New("el broker de acceso requiere LocalSystem")
	}
	return token, nil
}
func tokenSession(token windows.Token) (uint32, error) {
	var session, size uint32
	err := windows.GetTokenInformation(token, windows.TokenSessionId, (*byte)(unsafe.Pointer(&session)), 4, &size)
	return session, err
}
func noConsoleUser(session uint32) (bool, error) {
	var token windows.Token
	err := windows.WTSQueryUserToken(session, &token)
	if err == nil {
		token.Close()
		return false, nil
	}
	if errors.Is(err, windows.ERROR_NO_TOKEN) {
		return true, nil
	}
	return false, err
}

// ValidPreloginWorker is a fail-closed check before every privileged capture or
// input operation. Desktop name validation is additionally owned by the worker.
func ValidPreloginWorker() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err = validateInstalledExecutable(executable); err != nil {
		return err
	}
	token, err := systemToken(windows.TOKEN_QUERY)
	if err != nil {
		return err
	}
	defer token.Close()
	session, err := tokenSession(token)
	if err != nil {
		return err
	}
	active := windows.WTSGetActiveConsoleSessionId()
	if active == 0xffffffff || session == 0 || session != active {
		return errors.New("el broker no pertenece a la consola de acceso activa")
	}
	absent, err := noConsoleUser(active)
	if err != nil {
		return err
	}
	if !absent {
		return errors.New("sesión de usuario iniciada; detener broker de acceso")
	}
	return nil
}

// PreloginStopRequested allows the fixed child to release USB cooperatively.
// No external client is granted modification rights on this SYSTEM event.
func PreloginStopRequested() (bool, error) {
	name, _ := windows.UTF16PtrFromString(preloginStopEvent)
	event, err := windows.OpenEvent(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return true, err
	}
	defer windows.CloseHandle(event)
	state, err := windows.WaitForSingleObject(event, 0)
	return state == windows.WAIT_OBJECT_0, err
}

func enablePrivilege(token windows.Token, name string) (func(), error) {
	var luid windows.LUID
	p, _ := windows.UTF16PtrFromString(name)
	if err := windows.LookupPrivilegeValue(nil, p, &luid); err != nil {
		return nil, err
	}
	update := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}}}
	var previous windows.Tokenprivileges
	var size uint32
	if err := windows.AdjustTokenPrivileges(token, false, &update, uint32(unsafe.Sizeof(previous)), &previous, &size); err != nil {
		return nil, err
	}
	restore := func() { _ = windows.AdjustTokenPrivileges(token, false, &previous, 0, nil, nil) }
	// AdjustTokenPrivileges may succeed with ERROR_NOT_ALL_ASSIGNED; query the
	// resulting token instead of trusting the wrapper's success return.
	var needed uint32
	_ = windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &needed)
	if needed == 0 {
		restore()
		return nil, errors.New("no se pudieron verificar privilegios de servicio")
	}
	b := make([]byte, needed)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &b[0], needed, &needed); err != nil {
		restore()
		return nil, err
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&b[0]))
	for _, priv := range privileges.AllPrivileges() {
		if priv.Luid == luid && priv.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
			return restore, nil
		}
	}
	restore()
	return nil, fmt.Errorf("privilegio de servicio no disponible: %s", name)
}

type consoleChild struct {
	process windows.Handle
	session uint32
}

func stopConsoleChild(child *consoleChild, event windows.Handle) {
	if child == nil {
		return
	}
	_ = windows.SetEvent(event)
	state, err := windows.WaitForSingleObject(child.process, 3000)
	if err != nil || state == uint32(windows.WAIT_TIMEOUT) {
		// Only the process handle returned by our own CreateProcessAsUser call.
		_ = windows.TerminateProcess(child.process, 1)
		_, _ = windows.WaitForSingleObject(child.process, 1000)
	}
	_ = windows.CloseHandle(child.process)
}
func launchConsoleChild(token windows.Token, session uint32, executable string) (*consoleChild, error) {
	if err := validateInstalledExecutable(executable); err != nil {
		return nil, err
	}
	var childToken windows.Token
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_ALL_ACCESS, nil, windows.SecurityImpersonation, windows.TokenPrimary, &childToken); err != nil {
		return nil, err
	}
	defer childToken.Close()
	if err := windows.SetTokenInformation(childToken, windows.TokenSessionId, (*byte)(unsafe.Pointer(&session)), 4); err != nil {
		return nil, err
	}
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, childToken, false); err != nil {
		return nil, err
	}
	defer windows.DestroyEnvironmentBlock(env)
	app, _ := windows.UTF16PtrFromString(executable)
	command, _ := windows.UTF16FromString(`"` + executable + `" --prelogin-worker`)
	desktop, _ := windows.UTF16PtrFromString(`WinSta0\Winlogon`)
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(executable))
	startup := windows.StartupInfo{Desktop: desktop}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var process windows.ProcessInformation
	if err := windows.CreateProcessAsUser(childToken, app, &command[0], nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW, env, directory, &startup, &process); err != nil {
		return nil, err
	}
	windows.CloseHandle(process.Thread)
	return &consoleChild{process: process.Process, session: session}, nil
}

// launchStandardUserAgent uses the actual console user's primary token. The
// SYSTEM service never launches the ordinary GUI with its own privileged token.
func launchStandardUserAgent(session uint32, executable string) error {
	if err := validateInstalledExecutable(executable); err != nil {
		return err
	}
	if session == 0 || session == 0xffffffff || session != windows.WTSGetActiveConsoleSessionId() {
		return errors.New("consola de usuario no disponible")
	}
	var token windows.Token
	if err := windows.WTSQueryUserToken(session, &token); err != nil {
		return err
	}
	defer token.Close()
	// A console token can be a full UAC administrator token. Prefer its linked
	// filtered token so the ordinary GUI never inherits unnecessary elevation.
	// ElevationTypeDefault represents the user's actual non-split policy; do
	// not change Windows UAC policy or manufacture another account's token.
	var elevationType, size uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevationType, (*byte)(unsafe.Pointer(&elevationType)), 4, &size); err != nil {
		return err
	}
	if size != 4 || elevationType < 1 || elevationType > 3 {
		return errors.New("tipo de token de usuario no reconocido")
	}
	if elevationType == 2 { // TokenElevationTypeFull
		linked, err := token.GetLinkedToken()
		if err != nil {
			return err
		}
		defer linked.Close()
		var elevated uint32
		if err = windows.GetTokenInformation(linked, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &size); err != nil {
			return err
		}
		if size != 4 || elevated != 0 {
			return errors.New("el agente requiere el token UAC filtrado")
		}
		originalUser, err := token.GetTokenUser()
		if err != nil {
			return err
		}
		linkedUser, err := linked.GetTokenUser()
		if err != nil {
			return err
		}
		if !originalUser.User.Sid.Equals(linkedUser.User.Sid) {
			return errors.New("el token filtrado pertenece a otro usuario")
		}
		token = linked
	}
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	if user.User.Sid.IsWellKnown(windows.WinLocalSystemSid) {
		return errors.New("el agente de usuario no acepta un token LocalSystem")
	}
	// The console token can precede Userinit's profile load. Building an
	// environment at that point can pick the SYSTEM/default profile and hide
	// startup settings/USB diagnostics from the actual user. Wait; never race
	// Userinit by calling LoadUserProfile here.
	profile, profileErr := registry.OpenKey(registry.USERS, user.User.Sid.String(), registry.QUERY_VALUE)
	if profileErr != nil {
		return errors.New("perfil de usuario aún no disponible")
	}
	profile.Close()
	actual, err := tokenSession(token)
	if err != nil {
		return err
	}
	if actual != session {
		return errors.New("la sesión del agente cambió")
	}
	var env *uint16
	if err = windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return err
	}
	defer windows.DestroyEnvironmentBlock(env)
	// Recheck after environment construction. A failed query is never permission
	// to launch into an unknown or newly logged-off console.
	absent, err := noConsoleUser(session)
	if err != nil {
		return err
	}
	if absent || session != windows.WTSGetActiveConsoleSessionId() {
		return errors.New("la consola cerró la sesión")
	}
	app, _ := windows.UTF16PtrFromString(executable)
	command, _ := windows.UTF16FromString(`"` + executable + `" --startup`)
	desktop, _ := windows.UTF16PtrFromString(`WinSta0\Default`)
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(executable))
	startup := windows.StartupInfo{Desktop: desktop}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var process windows.ProcessInformation
	if err = windows.CreateProcessAsUser(token, app, &command[0], nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW, env, directory, &startup, &process); err != nil {
		return err
	}
	// This process belongs to the signed-in user. The service neither kills it on
	// stop nor restarts it after Exit; the GUI's singleton coalesces HKLM Run.
	windows.CloseHandle(process.Thread)
	windows.CloseHandle(process.Process)
	return nil
}

// SuperviseConsole launches only this protected executable on Winlogon's
// console desktop while no user token exists. It never opens a listener or
// accepts commands; sign-in immediately hands control back to the user agent.
func SuperviseConsole(ctx context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err = validateInstalledExecutable(executable); err != nil {
		return err
	}
	token, err := systemToken(windows.TOKEN_DUPLICATE | windows.TOKEN_ADJUST_PRIVILEGES)
	if err != nil {
		return err
	}
	defer token.Close()
	var restores []func()
	defer func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	}()
	for _, name := range []string{"SeTcbPrivilege", "SeAssignPrimaryTokenPrivilege", "SeIncreaseQuotaPrivilege"} {
		restore, err := enablePrivilege(token, name)
		if err != nil {
			return err
		}
		restores = append(restores, restore)
	}
	// Explicit SYSTEM-only access prevents another session signaling or replacing
	// the event and driving the privileged child lifecycle.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)")
	if err != nil {
		return err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	name, _ := windows.UTF16PtrFromString(preloginStopEvent)
	event, err := windows.CreateEvent(&attrs, 1, 0, name)
	if err != nil {
		if event != 0 {
			windows.CloseHandle(event)
		}
		return err
	}
	defer windows.CloseHandle(event)
	handoffSD, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;0x100000;;;AU)")
	if err != nil {
		return err
	}
	handoffAttrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: handoffSD}
	handoffName, _ := windows.UTF16PtrFromString(consoleHandoffEvent)
	handoff, err := windows.CreateEvent(&handoffAttrs, 1, 1, handoffName)
	if err != nil {
		if handoff != 0 {
			windows.CloseHandle(handoff)
		}
		return err
	}
	defer windows.CloseHandle(handoff)
	var child *consoleChild
	defer writeConsoleState("stopped", 0)
	defer func() { stopConsoleChild(child, event); _ = windows.SetEvent(handoff) }()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	var nextLaunch time.Time
	var userAgent userAgentLaunch
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		session := windows.WTSGetActiveConsoleSessionId()
		absent := false
		var tokenQueryErr error
		if session != 0xffffffff && session != 0 {
			absent, tokenQueryErr = noConsoleUser(session)
			// Transient query failures fail closed: stop the secure worker rather than
			// treat an inaccessible user's session as a pre-login console.
		}
		absent = eligibleConsole(session, absent, tokenQueryErr)
		if child == nil && !absent {
			if session == 0 || session == 0xffffffff || tokenQueryErr != nil {
				reportConsoleError("session_transition", tokenQueryErr)
			}
		}
		if child != nil {
			state, waitErr := windows.WaitForSingleObject(child.process, 0)
			if !absent || session != child.session || waitErr != nil || state == windows.WAIT_OBJECT_0 {
				var exitCode uint32
				_ = windows.GetExitCodeProcess(child.process, &exitCode)
				stopConsoleChild(child, event)
				_ = windows.SetEvent(handoff)
				if absent {
					writeConsoleState("worker_exited", exitCode)
				} else {
					writeConsoleState("waiting_login", 0)
				}
				child = nil
				nextLaunch = time.Now().Add(2 * time.Second)
			}
		}
		// Complete secure-worker shutdown before handing USB/capture to a
		// freshly created standard-user GUI. HKLM Run alone can be delayed or
		// disabled by Windows startup processing and is not a handoff guarantee.
		if child == nil && userAgent.due(session, absent, tokenQueryErr, time.Now()) {
			writeConsoleState("user_agent_starting", 0)
			launchErr := launchStandardUserAgent(session, executable)
			userAgent.result(launchErr, time.Now())
			if launchErr != nil {
				reportConsoleError("user_agent_error", launchErr)
			} else {
				writeConsoleState("user_agent_started", 0)
			}
		}
		if absent && child == nil && !time.Now().Before(nextLaunch) {
			if err = windows.ResetEvent(event); err != nil {
				return err
			}
			if err = windows.ResetEvent(handoff); err != nil {
				return err
			}
			writeConsoleState("launching", 0)
			child, err = launchConsoleChild(token, session, executable)
			if err != nil {
				_ = windows.SetEvent(handoff)
				reportConsoleError("launch_error", err)
				nextLaunch = time.Now().Add(2 * time.Second)
			} else {
				writeConsoleState("waiting_usb", 0)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// WaitForConsoleHandoff prevents the ordinary GUI from racing the privileged
// worker's USB handles and session-local capture mutex during user sign-in.
// Authenticated users can wait but cannot signal/reset this SYSTEM-owned event.
func WaitForConsoleHandoff(ctx context.Context) error {
	name, _ := windows.UTF16PtrFromString(consoleHandoffEvent)
	event, err := windows.OpenEvent(windows.SYNCHRONIZE, false, name)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		state, waitErr := windows.WaitForSingleObject(event, 100)
		if waitErr != nil {
			return waitErr
		}
		if state == windows.WAIT_OBJECT_0 {
			return nil
		}
		if state != uint32(windows.WAIT_TIMEOUT) {
			return fmt.Errorf("estado de transferencia de consola inválido: %d", state)
		}
	}
}
