# Windows virtual display v0.3

Goal: Windows extends its desktop to the existing Yam iOS app through a real virtual monitor. Preserve earlier desktop and Linux modes. User explicitly requested implementation and previously authorized autonomous execution through trial executables.

Decision: integrate pinned VirtualDrivers/Virtual-Display-Driver25.7.23 (MIT) as IddCx provider. Own driver compilation/signing cannot be completed on this Mac; an existing signed provider enables a testable native Windows executable. Do not change the driver's global monitor count, XML settings, trust stores or enforcement policy. No physical display selected as fallback.

Parallel ownership:
- Portable internal/winvirtual lease/state policy and tests: virtual_policy worker.
- Windows adapter identity, activation/restore and capture: virtual_windows worker.
- Verified embedded driver install using standard Windows policy: driver_installer worker.
- Main: CLI, cleanup-error reporting, launchers, notices, packaging and consolidation.

Steps:
- [x] Acquire provider from official release, verify published archive digest and retain provenance/license.
- [x] Installer validates assets/signature, stages driver and creates only its root node through standard Windows APIs; no certificate insertion or enforcement bypass.
- [x] Strict virtual-adapter identification; save state; place1280×720 virtual screen to right of physical desktop; capture only that screen.
- [x] Restore prior state on normal exit/error/cancel; protect physical displays and concurrent external changes.
- [x] Integrate --install-driver and --mode extend, native executable and launchers; tests/race/vet/crossbuild; independent review.
- [x] Deliver Windows virtual-display trial with exact distinction between code/build verification and Windows runtime verification.

Constraints: no Windows runtime/WDK available here. Driver installation requires Windows administrator privileges and normal OS signature acceptance. No promise of verified rendering/topology until tested on Windows. Installer is explicit; normal streaming does not install drivers.

Implemented with pinned third-party IddCx provider, not a new in-house driver. Review corrected unsupported1280x720 request: enumerate actual modes and prefer supported1920x1080 while scaling video to1280x720. Also corrected Y adjacency for stepped monitor layouts. Tests/race/vet and Windows cross-builds pass; Windows installation, signature trust, topology, capture and restoration remain runtime-unverified.
