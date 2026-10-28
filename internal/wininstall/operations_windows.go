//go:build windows

package wininstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\RelevoDisplay`
const markerName = "installation.json"
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "RelevoDisplay"

func InstalledExecutable() (string, error) {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Relevo Display", "Relevo-Windows.exe"), nil
}
func IsInstalledLocation() bool {
	target, err := InstalledExecutable()
	if err != nil {
		return false
	}
	exe, err := os.Executable()
	return err == nil && strings.EqualFold(filepath.Clean(exe), filepath.Clean(target))
}
func admin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(&windows.SECURITY_NT_AUTHORITY, 2, windows.SECURITY_BUILTIN_DOMAIN_RID, windows.DOMAIN_ALIAS_RID_ADMINS, 0, 0, 0, 0, 0, 0, &sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)
	ok, err := windows.Token(0).IsMember(sid)
	return err == nil && ok
}

// Check every existing path component: Program Files and its descendants must
// not redirect elevated writes through a junction/symlink/reparse point.
func safePath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		p, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attr, err := windows.GetFileAttributes(p)
		if err == nil && attr&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("ruta de instalación redirigida: %s", current)
		}
		if err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return err
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
	}
	return nil
}
func owned(root string) error {
	marker := filepath.Join(root, markerName)
	if err := safePath(marker); err != nil {
		return err
	}
	if err := verifyProtected(marker, false); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, markerName))
	if err != nil {
		return fmt.Errorf("no se reconoce la instalación existente: %w", err)
	}
	return validateManifest(data)
}
func copyStage(root, source string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	var magic [2]byte
	if _, err = io.ReadFull(in, magic[:]); err != nil || string(magic[:]) != "MZ" {
		return "", fmt.Errorf("ejecutable de instalación inválido")
	}
	if _, err = in.Seek(0, 0); err != nil {
		return "", err
	}
	out, err := os.CreateTemp(root, "relevo-stage-*.exe")
	if err != nil {
		return "", err
	}
	name := out.Name()
	_, err = io.Copy(out, in)
	err = errors.Join(err, out.Sync(), out.Close())
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
func moveReplace(from, to string) error {
	f, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	t, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(f, t, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
func register(target, version string) error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, uninstallKey, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return err
	}
	defer key.Close()
	values := map[string]string{"DisplayName": "Relevo Display", "DisplayVersion": version, "Publisher": "Relevo Display", "InstallLocation": filepath.Dir(target), "DisplayIcon": target, "UninstallString": installCommand(target, "--uninstall"), "ModifyPath": installCommand(target, "--setup")}
	for k, v := range values {
		if err := key.SetStringValue(k, v); err != nil {
			return err
		}
	}
	if err := key.SetDWordValue("NoModify", 0); err != nil {
		return err
	}
	if err := key.SetDWordValue("NoRepair", 0); err != nil {
		return err
	}
	run, _, err := registry.CreateKey(registry.LOCAL_MACHINE, runKey, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return err
	}
	defer run.Close()
	return run.SetStringValue(runName, installCommand(target, "--startup"))
}
func install(ctx context.Context, o Options, repair bool) (message string, err error) {
	target, err := InstalledExecutable()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(target)
	if err = safePath(target); err != nil {
		return "", err
	}
	_, statErr := os.Stat(target)
	exists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}
	if err = verifyProtected(filepath.Dir(root), true); err != nil {
		return "", err
	}
	if exists {
		if err = VerifyProtectedExecutable(target); err != nil {
			return "", err
		}
		if err = owned(root); err != nil {
			return "", err
		}
	} else if repair {
		return "", fmt.Errorf("no existe una instalación para reparar; usa Instalar")
	}
	if _, rootErr := os.Stat(root); rootErr == nil {
		if err = verifyProtected(root, false); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(rootErr) {
		return "", rootErr
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	if err = protectPath(root); err != nil {
		return "", err
	}
	marker := filepath.Join(root, markerName)
	if err = safePath(marker); err != nil {
		return "", err
	}
	if _, markerErr := os.Stat(marker); markerErr == nil {
		if err = verifyProtected(marker, false); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(markerErr) {
		return "", markerErr
	}
	source, err := os.Executable()
	if err != nil {
		return "", err
	}
	same := strings.EqualFold(filepath.Clean(source), filepath.Clean(target))
	var stage string
	if !same {
		stage, err = copyStage(root, source)
		if err != nil {
			return "", err
		}
		defer os.Remove(stage)
		if err = protectPath(stage); err != nil {
			return "", err
		}
	}
	if o.RemoveService == nil || o.InstallService == nil || o.Prepare == nil {
		return "", fmt.Errorf("faltan componentes de instalación")
	}
	var backup string
	var oldMarker []byte
	if exists && !same {
		backup, err = copyStage(root, target)
		if err != nil {
			return "", err
		}
		if err = protectPath(backup); err != nil {
			os.Remove(backup)
			return "", err
		}
		oldMarker, err = os.ReadFile(filepath.Join(root, markerName))
		if err != nil {
			os.Remove(backup)
			return "", err
		}
	}
	committed := false
	defer func() {
		if backup == "" {
			return
		}
		if committed {
			os.Remove(backup)
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		stopErr := o.RemoveService(rollbackCtx)
		var restoreErr error
		if stopErr == nil {
			restoreErr = moveReplace(backup, target)
		}
		if stopErr == nil && restoreErr == nil {
			restoreErr = errors.Join(os.WriteFile(filepath.Join(root, markerName), oldMarker, 0600), o.InstallService(target))
		}
		if stopErr != nil || restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("no se completó la restauración; copia anterior conservada en %s: %w", backup, errors.Join(stopErr, restoreErr)))
		}
	}()
	if exists && o.PrepareMaintenance != nil {
		if err = o.PrepareMaintenance(ctx); err != nil {
			committed = true
			return "", fmt.Errorf("no se modificó la instalación: %w", err)
		}
	}
	if err = o.RemoveService(ctx); err != nil {
		committed = true // No files replaced; keep the original running service intact.
		return "", fmt.Errorf("detener servicio anterior: %w", err)
	}
	// A locked user GUI must be closed through its own UI. Never terminate
	// unrelated processes and never schedule an ambiguous replacement at reboot.
	if !same {
		if err = moveReplace(stage, target); err != nil {
			committed = true // The destination was never replaced.
			if exists {
				_ = o.InstallService(target)
			}
			return "", fmt.Errorf("no se pudo reemplazar Relevo; cierra la sesión y selecciona Salir en su icono antes de actualizar: %w", err)
		}
	}
	writeMarker := func() error {
		b, _ := json.MarshalIndent(manifest{Product: "RelevoDisplay", Version: o.Version}, "", "  ")
		if err := os.WriteFile(filepath.Join(root, markerName), b, 0600); err != nil {
			return err
		}
		return protectPath(filepath.Join(root, markerName))
	}
	if err = writeMarker(); err != nil {
		return "", err
	}
	noticePath := filepath.Join(root, "THIRD_PARTY_NOTICES.txt")
	if err = safePath(noticePath); err != nil {
		return "", err
	}
	if _, e := os.Stat(noticePath); e == nil {
		if err = verifyProtected(noticePath, false); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if err = os.WriteFile(noticePath, thirdPartyNotices, 0644); err != nil {
		return "", err
	}
	if err = protectPath(noticePath); err != nil {
		return "", err
	}
	if err = o.Prepare(ctx); err != nil {
		return "", fmt.Errorf("archivos instalados; no se pudo preparar el controlador. Usa Reparar: %w", err)
	}
	if err = VerifyProtectedExecutable(target); err != nil {
		return "", err
	}
	if err = o.InstallService(target); err != nil {
		return "", fmt.Errorf("archivos instalados; no se pudo registrar el servicio. Usa Reparar: %w", err)
	}
	if err = writeShortcut(target); err != nil {
		return "", fmt.Errorf("no se pudo crear el acceso del menú Inicio: %w", err)
	}
	if err = register(target, o.Version); err != nil {
		return "", fmt.Errorf("instalación parcial; no se pudo registrar en Aplicaciones de Windows. Usa Reparar: %w", err)
	}
	committed = true
	return "Instalación preparada. Servicio registrado y controlador preparado. Puedes abrir Relevo. La captura anterior al inicio de sesión está incluida para prueba; comprueba su estado desde este GUI.", nil
}
func uninstall(ctx context.Context, o Options) (string, error) {
	target, err := InstalledExecutable()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(target)
	if err = safePath(target); err != nil {
		return "", err
	}
	if err = owned(root); err != nil {
		return "", err
	}
	if o.PrepareUninstall != nil {
		if err = o.PrepareUninstall(ctx); err != nil {
			return "", fmt.Errorf("no se desinstaló Relevo: %w", err)
		}
	}
	if o.RemoveService == nil {
		return "", fmt.Errorf("falta el administrador del servicio")
	}
	if err = o.RemoveService(ctx); err != nil {
		return "", err
	}
	run, e := registry.OpenKey(registry.LOCAL_MACHINE, runKey, registry.QUERY_VALUE|registry.SET_VALUE|registry.WOW64_64KEY)
	if e == nil {
		value, _, getErr := run.GetStringValue(runName)
		if getErr == nil && value == installCommand(target, "--startup") {
			err = run.DeleteValue(runName)
		}
		run.Close()
		if err != nil {
			return "", err
		}
	} else if !errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
		return "", e
	}
	if err = registry.DeleteKey(registry.LOCAL_MACHINE, uninstallKey); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return "", err
	}
	if err = removeShortcut(); err != nil {
		return "", err
	}
	// Keep the shared monitor and Apple drivers and user preferences. Deleting
	// the sole active display could leave the PC without an accessible desktop.
	if err = os.Remove(target); err != nil {
		oldMarker, readErr := os.ReadFile(filepath.Join(root, markerName))
		if readErr != nil {
			return "", readErr
		}
		pending, _ := json.Marshal(manifest{Product: "RelevoDisplay", Version: o.Version, PendingRemoval: true})
		if err = os.WriteFile(filepath.Join(root, markerName), pending, 0600); err != nil {
			return "", fmt.Errorf("servicio retirado; no se pudo marcar la desinstalación pendiente: %w", err)
		}
		p, _ := windows.UTF16PtrFromString(target)
		if err = windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err != nil {
			restoreErr := os.WriteFile(filepath.Join(root, markerName), oldMarker, 0600)
			return "", errors.Join(fmt.Errorf("servicio retirado; no se pudo retirar el ejecutable: %w", err), restoreErr)
		}
		return "Servicio y registro de aplicación retirados. El ejecutable se eliminará al reiniciar Windows. Se conservan los controladores compartidos y tus ajustes.", nil
	}
	// Retire only the installed license file, never redirected or user-replaced data.
	noticePath := filepath.Join(root, "THIRD_PARTY_NOTICES.txt")
	if safePath(noticePath) == nil && verifyProtected(noticePath, false) == nil {
		_ = os.Remove(noticePath)
	}
	_ = os.Remove(filepath.Join(root, markerName))
	_ = os.Remove(root)
	return "Relevo desinstalado. Se conservan los controladores compartidos y tus ajustes.", nil
}
func operation(o Options, action string) (string, error) {
	release, err := acquireInstallationLock()
	if err != nil {
		return "", err
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	switch action {
	case "install":
		return install(ctx, o, false)
	case "repair":
		return install(ctx, o, true)
	case "uninstall":
		return uninstall(ctx, o)
	}
	return "", fmt.Errorf("acción no válida")
}
