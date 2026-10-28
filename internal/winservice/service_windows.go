//go:build windows

package winservice

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type bootHandler struct {
	prepare   func(context.Context) error
	supervise func(context.Context) error
}

// Run serves SCM in session 0. It never opens windows, captures a desktop,
// injects input, or disables a monitor when the service stops.
func Run(prepare func(context.Context) error) error {
	if prepare == nil {
		return errors.New("boot preparation callback required")
	}
	return RunWithSupervisor(prepare, nil)
}

// RunWithSupervisor starts a bounded boot preparation then the console broker.
func RunWithSupervisor(prepare func(context.Context) error, supervise func(context.Context) error) error {
	if prepare == nil {
		return errors.New("boot preparation callback required")
	}
	return svc.Run(Name, &bootHandler{prepare: prepare, supervise: supervise})
}
func (h *bootHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (bool, uint32) {
	resetPreloginResult()
	ctx, cancel := context.WithCancel(context.Background())
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	defer bootCancel()
	defer cancel()
	state := svc.Status{State: svc.StartPending, CheckPoint: 1, WaitHint: 10000, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	updates <- state
	ready := make(chan error, 1)
	go func() { ready <- prepareWithRetry(bootCtx, h.prepare, 2*time.Second) }()
	var supervised <-chan error
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case err := <-ready:
			ready = nil
			if err != nil {
				updates <- svc.Status{State: svc.StopPending, WaitHint: 1000}
				return true, 1
			}
			bootCancel()
			if h.supervise != nil {
				ch := make(chan error, 1)
				supervised = ch
				go func() { ch <- h.supervise(ctx) }()
			}
			// Preparation is complete; do not keep touching PnP. In particular a GUI
			// Stop operation must not be undone by a background watchdog.
			state = svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
			updates <- state
		case <-supervised:
			updates <- svc.Status{State: svc.StopPending, WaitHint: 1000}
			return true, 2
		case <-tick.C:
			if state.State == svc.StartPending {
				state.CheckPoint++
				updates <- state
			}
		case request, ok := <-requests:
			if !ok {
				cancel()
				awaitSupervisor(supervised)
				return false, 0
			}
			switch request.Cmd {
			case svc.Stop, svc.Shutdown:
				updates <- svc.Status{State: svc.StopPending, WaitHint: 5000}
				cancel()
				awaitSupervisor(supervised)
				return false, 0
			case svc.Interrogate:
				updates <- state
			}
		}
	}
}

// Install creates/repairs the fixed service and starts it. The caller must have
// already written and validated the executable in protected Program Files.
func Install(executable string) error {
	if err := validateInstalledExecutable(executable); err != nil {
		return err
	}
	if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\"\x00\r\n") || !strings.EqualFold(filepath.Base(executable), "Relevo-Windows.exe") {
		return errors.New("invalid installed service executable")
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	config := mgr.Config{DisplayName: DisplayName, Description: "Prepara el adaptador virtual al arrancar y supervisa el proceso de consola previo al inicio de sesión.", StartType: mgr.StartAutomatic, ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, ServiceStartName: "LocalSystem", ErrorControl: mgr.ErrorNormal}
	command := `"` + executable + `" --service`
	s, err := m.OpenService(Name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		s, err = m.CreateService(Name, executable, config, "--service")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		old, configErr := s.Config()
		if configErr != nil {
			s.Close()
			return configErr
		}
		if old.BinaryPathName != command || old.DisplayName != DisplayName {
			s.Close()
			return errors.New("el nombre del servicio pertenece a otra instalación; no se modificó")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		stopErr := stop(ctx, s)
		cancel()
		if stopErr != nil {
			s.Close()
			return stopErr
		}
		config.BinaryPathName = command
		if err = s.UpdateConfig(config); err != nil {
			s.Close()
			return err
		}
	}
	defer s.Close()
	// Crash recovery is bounded by SCM. Do not reboot or launch shell commands.
	if err = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 10 * time.Second}, {Type: mgr.NoAction}}, 86400); err != nil {
		return err
	}
	if err = s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	return waitFor(ctx, s, svc.Running)
}
func waitFor(ctx context.Context, s *mgr.Service, want svc.State) error {
	for {
		status, err := s.Query()
		if err != nil {
			return err
		}
		if status.State == want {
			return nil
		}
		if want == svc.Running && status.State == svc.Stopped {
			return fmt.Errorf("el servicio no pudo preparar el adaptador (Windows %d, servicio %d)", status.Win32ExitCode, status.ServiceSpecificExitCode)
		}
		timer := time.NewTimer(150 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func stop(ctx context.Context, s *mgr.Service) error {
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	if st.State != svc.StopPending {
		if _, err = s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	return waitFor(ctx, s, svc.Stopped)
}

// Remove stops and removes only Relevo's own service. It leaves display and
// Apple driver packages installed, preserving the user's last usable output.
func Remove(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(Name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	defer s.Close()
	config, err := s.Config()
	if err != nil {
		return err
	}
	args, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil || len(args) != 2 || args[1] != "--service" || !strings.EqualFold(filepath.Base(args[0]), "Relevo-Windows.exe") || config.DisplayName != DisplayName {
		return errors.New("no se retiró un servicio de otra instalación")
	}
	if err = validateInstalledExecutable(args[0]); err != nil {
		return err
	}
	if err = stop(ctx, s); err != nil {
		return err
	}
	if err = s.Delete(); err != nil {
		return err
	}
	clearConsoleState()
	return nil
}

// Status uses query-only SCM permissions and is safe for the ordinary GUI.
func Status() (string, error) {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return "", err
	}
	defer windows.CloseServiceHandle(m)
	name, _ := windows.UTF16PtrFromString(Name)
	handle, err := windows.OpenService(m, name, windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return "Servicio no instalado", nil
	}
	if err != nil {
		return "", err
	}
	defer windows.CloseServiceHandle(handle)
	s := mgr.Service{Name: Name, Handle: handle}
	state, err := s.Query()
	if err != nil {
		return "", err
	}
	switch state.State {
	case svc.Running:
		return "Servicio activo · preparación de arranque completada\n" + readConsoleState() + "\n" + readPreloginResult(), nil
	case svc.StartPending:
		return "Servicio preparando el adaptador", nil
	case svc.StopPending:
		return "Servicio deteniéndose", nil
	case svc.Stopped:
		if state.Win32ExitCode != 0 || state.ServiceSpecificExitCode != 0 {
			return fmt.Sprintf("Servicio detenido con error (Windows %d, servicio %d)", state.Win32ExitCode, state.ServiceSpecificExitCode), nil
		}
		return "Servicio detenido", nil
	default:
		return fmt.Sprintf("Servicio: estado %d", state.State), nil
	}
}

// Wait for the broker to release USB and reap its own child before SCM reports
// Stopped. No unbounded wait on either a device callback or child shutdown.
func awaitSupervisor(done <-chan error) {
	if done == nil {
		return
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}
