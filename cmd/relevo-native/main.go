// Relevo Native is a developer entry point for the shared native-client host.
package main

import (
	"errors"
	"fmt"
	"os"
	"relevo/internal/jpegfast"
	"relevo/internal/nativeapp"
)

func main() {
	var err error
	if len(os.Args) != 1 || !nativeapp.LaunchGUI() {
		err = nativeapp.RunCLI()
	}
	if err = errors.Join(err, jpegfast.Shutdown()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
