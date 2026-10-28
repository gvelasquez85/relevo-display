# Managed Windows installation

Goal: one release EXE with graphical installation, repair/update and uninstall;
fixed protected Program Files location; automatic service startup and user agent.
Keep the tested Yam USB capture path unchanged.

Architecture: same signed-in-user GUI and one SCM service entry in the same EXE.
Service starts before login and enables the exact bundled virtual adapter once
per boot, recording bounded, non-sensitive status through SCM. It does not run GUI/capture in session 0. It supervises a fixed SYSTEM console
broker on WinSta0\Winlogon, only while no user token exists. The broker binds
its capture/input threads to Winlogon and authenticates the exact USB socket
owner as Apple Mobile Device Service. User capture is unprivileged; read-only
handoff synchronization prevents racing the privileged USB/capture handles.
Pre-login GDI image/input are implemented but not physically verified. Installer is elevated only for machine changes. Do not enable
Windows autologin, bypass secure desktops, kill arbitrary processes, remove
shared Apple drivers, or remove a user's only output on uninstall.

Implementation ownership:
- Windows service: internal/winservice, fixed entry arguments; boot preparation,
  bounded stop handling, install/update/delete and status APIs.
- Native graphical installer: internal/wininstall, fixed protected install dir,
  own files only, HKLM uninstall record, GUI install/repair/uninstall controls,
  rollback and no recursive deletion, service hooks passed by caller.
- Integration: cmd/relevo argument routing, normal setup-first downloaded copy,
  installed user GUI, service and driver hookup; unified build/package.

Validation: portable lifecycle/policy tests, race suite, Windows cross-vet and
GUI build, canonical ZIP/exe matching. Physical Windows install/repair/uninstall,
reboot/headless and UAC remain hardware validation through GUI. Explicitly show
Apple USB dependency status rather than imply it is bundled.
