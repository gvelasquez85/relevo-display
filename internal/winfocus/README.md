# Editable focus probe

`Probe(x, y int32) (bool, error)` receives physical desktop coordinates of a completed touch. The caller should request the iPad keyboard only on `true`, and hide it on `false` or an error. Invoke after injecting the touch-up, not on every move. No polling is required.

The Windows amd64 implementation checks the foreground thread's active/focused handles, foreground root, exact hit window and rectangle, control enabled state, native class and `ES_READONLY`. It snapshots focus again before returning. Password fields are eligible without ever reading their contents. Generic caret presence alone is deliberately insufficient: terminals, buttons and custom windows must not trigger the keyboard by accident.

Supported automatic detection: stock `Edit` and known `RichEdit` classes. Browser, Electron, UWP/WinUI, Office and custom controls are **not covered**. A later isolated UIAutomation helper can extend coverage; it must have a process-level timeout because providers can hang. The explicit keyboard mode remains necessary with those applications. Foreground changes immediately after the snapshot remain possible; callers should hide the keyboard on subsequent taps or session changes.

This version never calls `SendMessage`, text/accessibility value APIs, PowerShell or COM. A fixed number of User32 metadata queries avoids application message-loop waits, but Windows provides no strict wall-clock deadline for these OS calls. No detached timeout goroutine is used, so repeated probes cannot accumulate blocked goroutines. No window content, names, handles, coordinates or passwords are logged.

Verification: portable policy tests (editable, read-only, disabled, inactive, unrelated hit, unsupported/custom class); Windows amd64 cross-compilation and `go vet`. Physical Windows interaction, mixed-DPI and application compatibility remain unverified.

Primary references:

- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getguithreadinfo
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-guithreadinfo
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-windowfrompoint
- https://learn.microsoft.com/en-us/windows/win32/controls/edit-control-styles
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setthreaddpiawarenesscontext


## Browser edit fallback

`ProbeEditable` preserves `Probe` as its native fast path and, when that returns false, launches a hidden copy of the same executable in a narrowly scoped helper mode. No extra program is distributed. Only this child initializes UI Automation and calls providers. The caller terminates and waits for the child after 500 ms; a hung provider cannot leave a background probe accumulating in the host. Call this API on the serialized keyboard worker, never on the video/control hot loop. Process startup and provider availability may cause false negatives on slow hosts; native fast-path behavior is unchanged.

The helper queries only control type, enabled/focus/focusable flags, an explicitly supported ValuePattern read-only flag, and bounding rectangle. It requires a focused Edit control, a touch inside its rectangle, foreground and native focus stability, and UIA element identity stability. Unsupported readonly metadata, whole Document controls and readonly edits fail closed. This covers browser address bars and browser input controls that expose the Edit and writable ValuePattern metadata; arbitrary custom contenteditable providers are not guaranteed. No names, text, values, passwords, clipboard, URL, window titles or text ranges are read or logged. Runtime Windows/browser behavior still requires physical validation.

UIA interface order and ABI were checked against Microsoft's generated Windows SDK header:
https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/RecompiledIdlHeaders/um/UIAutomationClient.h

Properties and unsupported default behavior:
https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-propertiesforclients
