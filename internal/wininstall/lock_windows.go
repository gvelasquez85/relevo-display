//go:build windows

package wininstall

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// The handle's existence serializes complete transactions across setup windows
// and sessions, including their rollback. It is not a thread-owned mutex lock.
func acquireInstallationLock() (func(), error) {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)")
	if err != nil {
		return nil, err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	name, _ := windows.UTF16PtrFromString(`Global\RelevoDisplay.ManagedInstallation`)
	h, err := windows.CreateMutex(&attrs, false, name)
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("otra instalación está en curso o Windows no permite el bloqueo: %w", err)
	}
	return func() { windows.CloseHandle(h) }, nil
}
