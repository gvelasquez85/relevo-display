//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"relevo/internal/desktop"
	"relevo/internal/session"
	"relevo/internal/usbmux"
	"relevo/internal/windriver"
	"relevo/internal/wininstall"
	"relevo/internal/winsecure"
	"relevo/internal/winservice"
	"relevo/internal/wintray"
	"relevo/internal/yaminput"
	"strings"
	"syscall"
	"time"
)

func openManagedSetup() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--setup")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
func runManagedSetup(action string) {
	err := wininstall.Run(wininstall.Options{
		Version: version, Action: action,
		Prepare:            func(ctx context.Context) error { return windriver.Install(ctx, nil) },
		PrepareMaintenance: wintray.CloseForMaintenance,
		PrepareUninstall: func(ctx context.Context) error {
			if err := desktop.PrepareVirtualMonitorStop(ctx, nil); err != nil {
				return err
			}
			return wintray.CloseForMaintenance(ctx)
		},
		InstallService: func(path string) error {
			if err := winservice.Install(path); err != nil {
				return err
			}
			return wintray.RemoveLegacyLoginStartup()
		}, RemoveService: winservice.Remove, ServiceStatus: winservice.Status,
		DependencyStatus: func() string {
			_, err := (&usbmux.Client{}).Devices()
			if err != nil {
				return "USB de Apple: servicio no disponible. El controlador USB de Apple no está incluido en este instalador."
			}
			return "USB de Apple: servicio disponible. Abre la app compatible en el iPad."
		},
		Launch: func(path string) error {
			cmd := exec.Command(path)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return cmd.Start()
		},
	})
	if err != nil {
		wintray.ShowError(err)
	}
}
func handleManagedLaunch() bool {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "--service":
			_ = winservice.RunWithSupervisor(func(ctx context.Context) error { return windriver.SetDeviceEnabled(ctx, true, nil) }, winservice.SuperviseConsole)
			return true
		case "--prelogin-worker":
			runPreloginWorker()
			return true
		case "--setup":
			runManagedSetup("")
			return true
		case "--uninstall":
			runManagedSetup("uninstall")
			return true
		case "--app":
			launchDesktopApp()
			return true
		}
		if strings.HasPrefix(os.Args[1], "--setup-elevated=") {
			runManagedSetup(strings.TrimPrefix(os.Args[1], "--setup-elevated="))
			return true
		}
	}
	if len(os.Args) == 1 && !wininstall.IsInstalledLocation() {
		runManagedSetup("")
		return true
	}
	return false
}
func runPreloginWorker() {
	if winservice.ValidPreloginWorker() != nil || winsecure.Enable() != nil {
		winservice.ReportPreloginState("desktop_unavailable")
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				requested, err := winservice.PreloginStopRequested()
				if requested || err != nil || winservice.ValidPreloginWorker() != nil {
					close(stop)
					return
				}
			}
		}
	}()
	// Fixed settings: no user-writable file, Wi-Fi listener, contextual UIA,
	// keyboard GUI or diagnostic files in a privileged sign-in process.
	settings := wintray.DefaultSettings()
	settings.FPS = 30
	settings.Quality = 70
	settings.Keyboard = true
	settings.Touch = true
	settings.Mode = "extend"
	for {
		select {
		case <-stop:
			return
		default:
		}
		if winservice.ValidPreloginWorker() != nil {
			return
		}
		winservice.ReportPreloginState("waiting_usb")
		_ = runCompatibleSession(settings, stop, func(string) {}, compatibleRunOptions{Prelogin: true,
			Observe: func(st session.Stats, _ desktop.CaptureStatus) string {
				if st.FramesSent > 0 {
					winservice.ReportPreloginState("streaming")
				}
				return ""
			},
			Complete: func(st session.Stats, _ desktop.CaptureStatus, _ bool, err error, _ yaminput.Stats, _ bool) {
				if err != nil {
					if !errors.Is(err, wintray.ErrWaitingForDevice) {
						winservice.ReportPreloginState("capture_error")
					} else {
						winservice.ReportPreloginState("usb_error")
					}
				}
			},
		})
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
