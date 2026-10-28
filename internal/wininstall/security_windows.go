//go:build windows

package wininstall

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const trustedInstallerSID = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"

func trustedOwner(sid *windows.SID, systemParent bool) bool {
	if sid == nil {
		return false
	}
	return sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) || (systemParent && sid.String() == trustedInstallerSID)
}
func verifyProtected(path string, systemParent bool) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if !trustedOwner(owner, systemParent) {
		return fmt.Errorf("propietario no protegido de instalación: %s", path)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if acl == nil {
		return fmt.Errorf("carpeta o ejecutable con permisos abiertos: %s", path)
	}
	const dangerous uint32 = 0x2 | 0x4 | 0x10 | 0x100 | 0x40 | 0x10000 | 0x40000 | 0x80000 | 0x40000000 | 0x10000000
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err = windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if uint32(ace.Mask)&dangerous != 0 && !trustedOwner(sid, true) {
				return fmt.Errorf("permisos de escritura no protegidos en %s", path)
			}
		default:
			return fmt.Errorf("permisos de instalación no reconocidos en %s", path)
		}
	}
	return nil
}
func protectPath(path string) error {
	sd, err := windows.SecurityDescriptorFromString("O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;BU)")
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, owner, nil, acl, nil)
}

// VerifyProtectedExecutable is also used before registering/executing a SYSTEM
// service. Existing user-writable binaries are refused rather than merely fixed:
// a previously opened writable handle could survive a DACL change.
func VerifyProtectedExecutable(path string) error {
	expected, err := InstalledExecutable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(path), filepath.Clean(expected)) {
		return fmt.Errorf("servicio fuera de la instalación protegida")
	}
	if err = safePath(path); err != nil {
		return err
	}
	if err = verifyProtected(filepath.Dir(filepath.Dir(path)), true); err != nil {
		return err
	}
	if err = verifyProtected(filepath.Dir(path), false); err != nil {
		return err
	}
	return verifyProtected(path, false)
}
