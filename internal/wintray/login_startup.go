package wintray

import (
	"fmt"
	"strings"
)

// loginStartupCommand quotes the executable independently from the fixed flag.
// An executable path cannot contain quotes or NUL on Windows.
func loginStartupCommand(executable string) (string, error) {
	if executable == "" || strings.ContainsAny(executable, "\"\x00\r\n") {
		return "", fmt.Errorf("ruta de Relevo inválida para inicio automático")
	}
	return `"` + executable + `" --startup`, nil
}

func hasStartupFlag(args []string) bool {
	if len(args) < 2 {
		return false
	}
	for _, arg := range args[1:] {
		if arg == "--startup" {
			return true
		}
	}
	return false
}
