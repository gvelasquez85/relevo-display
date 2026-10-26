# Windows polish and performance

User reports good image but no visible GUI/icon/settings and slow cursor. Code evidence: v0.4 has hidden nativewindow and generic notificationicon, fixed5FPS GUI, JPEG80 and perframeGDIallocation/pixelresample.

Authorized implementation continuing prior autonomy preference:
- Native visible controlwindow and customtrayicon; resolution,FPS,quality controls persisted. Default20FPS quality70, choices10/15/20/30FPS with lower quality available. No performance guarantee for oldiPad.
- JPEG exactsize fastpath and reusable Windows capture resources, preserving UAC/cursor and cleanup.
- Session metrics elapsed, capture/write times and actualsendFPS, sendrate is not devicepresentationFPS.
- Build0.5GUIexe+ZIP, portable/race tests/vet/crosscompile and benchmark evidence. Independent finalreview. Windowsruntime remains user validation.

Owners: v05_gui wintray; v05_capture_perf desktop; root main/session/build/docs/integration.
