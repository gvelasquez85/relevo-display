//go:build !windows

package wintray

import "fmt"

func RunLab(o LabOptions) error { return fmt.Errorf("la ventana de pruebas requiere Windows") }
