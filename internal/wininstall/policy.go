// Package wininstall owns the graphical, managed Windows installation.
package wininstall

import (
	"context"
	"encoding/json"
	"fmt"
)

type Options struct {
	Version            string
	Action             string
	Prepare            func(context.Context) error
	PrepareUninstall   func(context.Context) error
	PrepareMaintenance func(context.Context) error
	DependencyStatus   func() string
	InstallService     func(string) error
	RemoveService      func(context.Context) error
	ServiceStatus      func() (string, error)
	Launch             func(string) error
}

func validateAction(action string) error {
	switch action {
	case "", "install", "repair", "uninstall":
		return nil
	}
	return fmt.Errorf("acción de instalación desconocida")
}

func installCommand(executable string, arg string) string { return `"` + executable + `" ` + arg }

type manifest struct {
	Product        string
	Version        string
	PendingRemoval bool `json:"pending_removal,omitempty"`
}

func validateManifest(data []byte) error {
	var m manifest
	if json.Unmarshal(data, &m) != nil || m.Product != "RelevoDisplay" {
		return fmt.Errorf("la carpeta no pertenece a una instalación Relevo reconocida")
	}
	if m.PendingRemoval {
		return fmt.Errorf("la desinstalación está pendiente: reinicia Windows antes de instalar de nuevo")
	}
	return nil
}
