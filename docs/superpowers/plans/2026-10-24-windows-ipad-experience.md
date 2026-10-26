# Windows iPad experience Implementation Plan

**Goal:** Deliver a Windows executable with 4:3 extended monitor, visible cursor, recoverable secure-desktop pauses, and tray controls.
**Architecture:** Keep USB Yam protocol and signed MTTVDD binary. Configure explicit iPad modes, negotiate the same JPEG dimensions, and expose native tray UI with elevated setup. Preserve Apple transport dependency with guided detection.
**Tech Stack:** Go, Win32 GDI/notification icons/SetupAPI, existing signed IddCx provider.
**Authorization:** User requested these behaviors and previously explicitly opted out of permission questions. Continue in current shared workspace with scoped file ownership.

- [x] Exact virtual resolutions: 1024x768 default; 1280x960,1600x1200,2048x1536 choices. Never choose primary resolution as fallback.
- [x] Match Yam handshake dimensions and JPEG output; test custom handshake and invalid dimensions.
- [x] Draw cursor with hotspot and correct coordinate scaling. Pause capture during secure desktop; preserve USB session and resume afterward.
- [x] Tray executable: start/stop, preset selector, install/configure driver with UAC, display settings and guided Apple dependency. Avoid BAT in default user flow.
- [x] Support persistent tray sessions until explicit stop, bounded command-line tests remain available.
- [x] Verify portable tests/race, Windows crosscompile/vet/ABI, package CRC/hashes; clearly mark Windows runtime testing pending.

## Review focus
Driver XML migration must not overwrite arbitrary user settings; signed binaries remain unchanged. Secure desktop protection stays enabled. Physical display topology and primary mode remain untouched. Tray exit waits for capture cleanup. Missing USB/device yields actionable UI. No claim that Apple transport dependency has been removed or installer is production signed.

Completed implementation and automated verification. Windows runtime acceptance remains pending and is recorded in analysis/windows-ipad-v04-validation.json.
