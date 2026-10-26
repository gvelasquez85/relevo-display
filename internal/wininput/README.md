# Extended native input (v0.5)

`NewExtended(target, backend)` accepts the same capture-bound `Target` callback as
legacy single-finger input. The callback must validate display identity and the
interactive desktop on every call; `desktop.InputTarget` supplies this policy.
Use `NativeExtendedBackend`, `HandleContacts`, `HandleKey`, and always `Close`
**before** restoring/detaching the display. Do not run legacy mouse conversion for
the same contact stream.

Touch uses `InitializeTouchInjection`/`InjectTouchInput`, not mouse synthesis.
Changed-contact batches are validated atomically by `remoteinput.State`; active
contacts get stable bounded Windows IDs and complete Windows frames. Coordinates
are physical desktop pixels, including negative monitor origins. UP/CANCEL retain
the last injected point (an API requirement). Replacement fingers first release
old slots; no frame exceeds ten contacts. A 50 ms stationary update supports
press-and-hold and revalidates topology while fingers are stationary. Letterbox
contacts are ignored until lifted; leaving the image cancels an active finger.
Topology/desktop errors end input and attempt to release all contacts and keys.

Text is committed UTF-8 translated into UTF-16 `KEYEVENTF_UNICODE` down/up pairs,
including surrogate pairs. Physical key codes support Control/Alt/Shift/Meta,
navigation/editing keys, `KeyA`..`KeyZ`, `Digit0`..`Digit9`, `Space`, `F1`..`F12`.
Use physical codes for shortcuts (e.g. Control down, KeyC press, Control up), not
Unicode text. Windows applies shortcuts to its current foreground application.
Partially applied SendInput batches release unmatched downs; failed releases are
retained by the native backend for retry during Close. Cleanup errors are returned,
not hidden. UIPI/secure-desktop boundaries can reject injection and cleanup;
there is no elevation or boundary bypass.

Windows x64 ABI sizes/offsets, packet conversion, simultaneous contacts, release
ordering, topology failure, stationary updates, Unicode/surrogates, letterbox
handling, ten-contact replacement, partial-batch release and control keys are
covered by portable tests. Windows amd64 crosscompile/vet were run; **physical
Windows/iPad behavior, application-specific gestures and accessibility behavior
still require device tests**. Native touch makes Windows gestures available to
applications that support them; it does not reproduce all iOS system gestures.

Microsoft references consulted 25 October 2026:

- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-injecttouchinput
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-pointer_info
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-pointer_touch_info
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-keybdinput
