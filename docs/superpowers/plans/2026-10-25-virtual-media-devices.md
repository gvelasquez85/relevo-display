# Relevo virtual media devices

User authorizes camera and microphone selectable by Windows apps, one eventual installer, no OBS. Existing own iOS 9.3.5 client supplies JPEG 640x480 and mono PCM16 22050 Hz. Own client signing/trust remains unresolved.

Implementation choices: camera via Windows 11 Media Foundation software camera source; microphone via a dedicated WaveRT/PortCls endpoint adapted from SYSVAD, not a render monitor. Camera broker identity/ACL must be validated on Windows before enabling transport. Audio endpoint needs a WDK build and accepted driver signature before normal distribution.

- Camera specialist owns source adapter, bounded latest frame and fail-closed IPC client; no changes to vendor originals.
- Microphone specialist owns portable conversion/buffer components and kernel-compatible feeder contract with tests; no fake driver endpoint claim.
- Parent owns CMake integration and Windows build preparation, final verification and limits documentation.

Acceptance this increment: deterministic source preparation tests; portable C++ ASan/UBSan checks for malformed payloads, stale/disconnect camera and audio continuity/underflow; reproducible Windows build instructions/script. Actual COM activation, Camera Frame Server identity/ACL, signed microphone endpoint and physical media validation remain required before claiming selectable devices.
