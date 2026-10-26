# Native Windows GUI test panel plan

Goal: run every native prototype test from Windows GUI/tray without user commands or BAT files.
Architecture: reuse native Win32 shell helpers, new test panel and shared native session runner; no child shell/process or global CLI flag mutation. Existing legacy GUI unchanged. Profile controls call same verified session code and stop through context cancellation and cleanup.

- [x] LabSettings/Profile validation tests then Win32 panel with six direct-start tests, FPS/quality, stop, diagnostics, tray and orderlyexit. Worker owns internal/wintray/lab* only.
- [x] Parent refactor nativeCLI into runTrial(ctx,config,report) callable by GUI; configure profiles from validatedLabSettings and tests ensuring chosenfeature flags. New zeroargsWindows entry opensGUI. Keep CLI for developers.
- [x] Coalesced live frame/input/mic counts; only count/metadata, no inputvalues or capturedmedia logs. Camera preview opens only followingexplicitcamera test; noOBSdependency.
- [x] Windowsamd64 GUI build, model/runner tests, go-race/vet and packaging. Do not claim WindowsUI visual/runtime testing from Mac. Camera systemdevice/micdevice andWiFipendinglabels.
