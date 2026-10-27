//go:build !windows

package wintray

import "fmt"

type Options struct {
	PrepareStop       func() error
	OpenKeyboard      func() error
	DisconnectMonitor func() error
	OpenYamTests      func(Settings)
	OpenTests         func()
	ExportDiagnostic  func() error
	Title             string
	Start             func(Settings, <-chan struct{}, func(string)) error
	Install           func() error
}

func Run(Options) error      { return fmt.Errorf("notification shell is available on Windows") }
func InstallElevated() error { return fmt.Errorf("Windows required") }
func ShowResult(error)       {}

func ShowError(error) {}
