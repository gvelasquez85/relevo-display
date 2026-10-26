//go:build !windows

package nativeapp

// LaunchGUI opens the native-client controls on Windows and reports whether handled.
func LaunchGUI() bool { return false }
