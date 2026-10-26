# Virtual devices development update — 2026-10-25

Current implementation adds real-frame adaptation to the COM camera source and microphone feeder components. Neither is installed or runtime validated as a selectable Windows device. The existing GUI preview/monitor remains separate.

- Camera: generated source replaces gradient generation with a bounded JPEG reader, proportional BGRX canvas and NV12 conversion inherited from the sample. Missing/disconnected/stale (>1s) input becomes black. The pipe reader is **OFF by default**, pending host server implementation and Camera Frame Server identity/ACL validation on Windows. Enable only explicitly in a development build with `RELEVO_ENABLE_EXPERIMENTAL_CAMERA_PIPE`; it is not a production transport.
- Host: `internal/mediabridge` now serializes and validates RVM1 camera/microphone/disconnect packets. It does not open a pipe or activate any device.
- Microphone: `src/microphone_feeder.hpp` converts 22050 Hz mono PCM16 into 48000 Hz mono PCM16 with continuous interpolation, a 120ms maximum queue and stale/disconnect silence. `src/microphone_endpoint.h` is C11 fixed-storage code with strict RVM2 feeder validation and session generation checks. It does not register an endpoint or implement a WaveRT miniport. Integration details: `docs/microphone-virtual-device.md` at repository root.
- Build: `tools/build-windows.ps1` compiles and runs unit tests in an x64 Visual Studio developer shell, given explicit WIL/C++WinRT header paths. It does not install or register anything. Windows SDK/WDK compilation and runtime acceptance remain outstanding.

Portable tests cover camera bounds/aspect padding, microphone packet split continuity, conversion counts, malformed input, overflow, silence, disconnect/reconnect, concurrent access and kernel boundary parsing. CMake includes both new test targets.

The older baseline notes below describe provenance and prerequisites. Any statement describing a synthetic gradient or an entirely unimplemented microphone feeder is superseded by this update; the selectable microphone endpoint remains unimplemented.

---

# Relevo Windows media components — development, not an installed device

25 October 2026. Owner: windows_media_v05. Windows 11 build 22000+ camera target, iPad iOS 9.3.5 input. No OBS runtime or paid dependency is introduced.

## Delivered and verified

- `src/media_bridge.hpp`: strict binary packet decoder and bounded queues. The caller decodes before enqueueing; camera dimensions require WIC validation. Camera queue is latest-only; microphone queue retains at most six packets (at most 120 ms at nominal 20 ms). Disconnect atomically clears both. No disk recording.
- `tests/media_bridge_test.cpp`: executes on Mac with clang++ (C++17), AddressSanitizer and UndefinedBehaviorSanitizer. Tests format/size/malformed rejection, oldaudio drop, camera replacement, disconnect flushing. Passing these tests does **not** validate Windows APIs.
- `src/camera_control.cpp`: real MFCreateVirtualCamera lifecycle utility with `start`/`remove`, current-user scope, persistent lifetime, fixed Relevo source CLSID. It refuses to create a camera when the COM source is absent. It does not register a COM class. The optional provider build described below supplies a synthetic IMFMediaSource baseline. It has **not compiled or run on Windows**.
- `src/camera_decode.hpp`: WIC JPEG → bounded640×48032bppBGR decode adapter for a future media source. Rejects othercontainers, multipleimages and oversized dimensions beforeoutputallocation. Requires COM MTA initialized by caller. Not Windows-compiled yet.

## Important capability boundaries

| Capability | Current state | Remaining requirement |
|---|---|---|
| RVL1 camera/mic reception | Host/client work outside this directory | Integration by host owner |
| Bounded media codec/queue | Implemented, portable tests passing | Wire transport adapter |
| WIC JPEG decode | Source implemented, not Windows compiled | SDK build + malformedJPEG tests on Windows |
| Camera registration lifecycle | Source implemented, not Windows compiled | Installed COM source + SDK build/runtime |
| COM camera source | Pinned official synthetic source + adapted DLL build target | Windows compile/activation tests; iPad JPEG feed and frame-server IPC identity/ACL |
| Selectable Relevo Camera in Teams/browser | **Not runtime validated** | Protected installation/registration and application tests |
| Selectable Relevo Microphone | **Not implemented** | Signed audio endpointdriver and feeder, kernel ringbuffer, format conversion, installation and tests |
| Audio Windows→iPad | Existing WASAPI loopback path | Physical simultaneousaudio/mic test; no virtualspeaker implied |
| Mic→PC playback/diagnostics | Host may consume PCM | **Does not create a selectable Windows microphone** |
| One installer without OBS | Targetarchitecture | Packaging after real providers are built/signed/tested |

## Local protocol proposal RVM1 (not yet wired to host)

This is a separate proposed local binary framing between host and future media broker; **it does not change RVL1**. It must not be exposed on TCP/LAN. No namedpipe is created by this package. Integration must choose a Windows pipe ACL and authenticate clients using OS identity after identifying actual CameraFrameServer/service identities. Do not copy an Everyone/AnonymousACL from samplecode.

24byteheader, littleendian, no implicitpadding:

| Offset | Type | Value |
|---|---|---|
|0|4bytes|ASCII `RVM1`|
|4|u16|version1|
|6|u16|kind1cameraJPEG,2micPCM,3disconnect|
|8|u32|sequence, advisory across one session|
|12|u32|payloadbytes|
|16|u64|hostmonotonic timestampµs; iPadclock must not be used directly|

Camera payload: JPEG<=512KiB, maximum 640×480, nominal 10 FPS. The codec checks SOI/EOI only; WIC must validate fullJPEG and dimensions. Camera source must emit a newlygeneratedblack/disconnectedframe after staleinput, never hold the last cameraframe indefinitely. No implicitcameraactivation.

Microphone payload: RVL1PCM unchanged: LEu32rate22050,LEu16channels1,LEu16bits16 followed by PCM16LE. 2..882samplebytes, even; total10..890bytes. Nominal441samples/20ms. A future48kHzWindowsendpoint must resample; do not relabel22050 as48000. On underflow/disconnect endpoint must emit silence with advancingtimestamps; microphoneconsent is clientcontrolled and is not bypassed.

Kind3 payloadempty, clears retainedaudio/video. Sessionboundary must be represented by a freshbrokerconnection; sequencewrap has no securitymeaning. Oneproducer per session, no raw input logs or caching.

## Build and check

Portable verification (run from repositoryroot):

```
clang++ -std=c++17 -Wall -Wextra -Werror -fsanitize=address,undefined \
 -I drivers/windows-media/src drivers/windows-media/tests/media_bridge_test.cpp \
 -o /tmp/relevo-media-bridge-test
/tmp/relevo-media-bridge-test
```

Windows developer shell: Visual Studio C++ + Windows SDK 22000 or newer + CMake. These are build dependencies, **not intended end user dependencies**.

```
cmake -S drivers/windows-media -B drivers/windows-media/build -A x64
cmake --build drivers/windows-media/build --config Release
ctest --test-dir drivers/windows-media/build -C Release --output-on-failure
```

`RelevoCameraControl start` deliberately fails with REGDB_E_CLASSNOTREG until a provider is installed. Do not ship this as a camera executable. CMake also compiles `RelevoCameraDecode` against windowscodecs/ole32. No provider currently consumes that adapter.

Current Mac has Apple clang but no clang-cl, MinGW g++, WindowsSDK, MSVC or WDK; Go crossbuild cannot prove C++/COMproviderbuilds. Nothing in this package installs a kernel driver, changes Secure Boot, enables TESTSIGNING or alters camera/microphone privacy settings.

## Provider implementation path

Microsoft's official sample supplies the concrete synthetic COM source described below. The next source task is replace its gradient generator with our bounded JPEG mailbox after validating the local broker ACL and identity. Installation must stop/remove the camera before unregistering COM; rollback must affect only Relevo resources.

Microphone is a separate problem. SYSVAD is an MIT PortCls/WaveRT sample, not a generic signed cable ready for redistribution. Its sample capture data and endpoints do not automatically consume USB PCM. A Relevo adaptation must exposeone capture endpoint and a restrictedfeeder interface, clock the capture buffer, convert PCM rates, bound queues, silence underflow and stop feeding on consent withdrawal. WDK build plusMicrosoft accepted driver signing / compatibility work are required beforeintegration intoan end user installer. No promised free production driver route and no third party redistribution rights beyond sample MIT are inferred.

A single final installer can containuser-mode camera provider and signed audio endpoint package, alongsideGUI/displaydriver. It does not makeaudio signing orApple provisioningdisappear. Do not substituteWASAPIrendering, an MJPEG browser page orOBS for a Windows capture device andcallitdone.

## Primary sources

- [MFCreateVirtualCamera](https://learn.microsoft.com/en-us/windows/win32/api/mfvirtualcamera/nf-mfvirtualcamera-mfcreatevirtualcamera): Windowsbuild22000+, source CLSID, currentuser/alluser andlifecycle.
- [Microsoftcamera sample](https://github.com/microsoft/Windows-Camera/tree/master/Samples/VirtualCamera): requiredsourceDLL, installer, streamimplementation andruntimeprerequisites.
- [MicrosoftSYSVAD](https://github.com/microsoft/Windows-driver-samples/tree/main/audio/sysvad): WDKsample, signingrequirements andbuildinstructions. Its testsigninginstallationinstructions are **not executed or recommended for this user's machine**.
- [Microsoftaudio samples](https://learn.microsoft.com/en-us/windows-hardware/drivers/audio/sample-audio-drivers).
- [Driver signing options](https://learn.microsoft.com/en-us/windows-hardware/drivers/dashboard/driver-signing-offerings):productionrequirements must becheckedbeforedistribution.

## Additional concrete delivery: actual COM source baseline

The previous lifecycle-only gap is now narrowed: `vendor/microsoft-camera` contains the **actual 14-file minimal source subset** from the pinned Microsoft camera sample. It includes IMFActivate, IMFMediaSourceEx, IMFMediaStream2, classfactory and DLL exports, not empty stubs. Original source bytes and MIT license are retained; `UPSTREAM.json` pins every source SHA256.

`tools/prepare_camera_source.py` verifies all retained hashes before producing adapted copies in the build directory. Adaptation selects the synthetic source only, removes physicalcamera wrappers, changes the source CLSID to match `RelevoCameraControl`, changes advertised rates/duration to640×48010FPS (RGB32/NV12), checks DllGetClassObject outputpointer and makes ShutdownObject shut down the source. The original sample's syntheticgradient remains the framegenerator. **This source is a syntheticcamera baseline, not an iPadwebcam yet.** RequestSample scheduling and allocator behavior remain inherited from Microsoft and require Windowsvalidation.

Build the real DLL on Windows with MSVC/SDK22000+ and the following build-only header dependencies:

- WIL, upstream sample pins NuGet `Microsoft.Windows.ImplementationLibrary` version`1.0.260126.7`.
- C++/WinRT, upstream pins `Microsoft.Windows.CppWinRT` version`3.0.260520.1`.
- Headerpaths must be supplied explicitly; this project does not silently download or execute NuGet packages. Use the package's include directory containing `wil/cppwinrt.h`, and C++/WinRT's include directory containing `winrt/base.h` plus Windows projections. Dependencies are header-only here, not separate user apps.

```
cmake -S drivers/windows-media -B drivers/windows-media/build -A x64 ^
 -DRELEVO_BUILD_CAMERA_PROVIDER=ON ^
 -DRELEVO_WIL_INCLUDE=C:/build-deps/wil/include ^
 -DRELEVO_CPPWINRT_INCLUDE=C:/build-deps/cppwinrt/include
cmake --build drivers/windows-media/build --config Release --target RelevoCameraSource RelevoCameraControl
```

Neither command installs anything. No Windowsbuild has run in this Mac environment. A futureinstaller must install the compiled DLL in a protectedProgramFiles location, register its `InprocServer32` under CLSID`{C869639B-87A0-49DC-951B-E53D34F059BA}` with `ThreadingModel=Both`, then invoke start; rollback/remove must remove the camera before unregistering/deleting the source. Registrychanges are deliberately not included in an automaticdeveloperinstallscript while Windowsvalidation is pending.

Portable source preparation checks:

```
python3 drivers/windows-media/tests/test_camera_source.py
```

These verify deterministicgeneration and rejection of tamperedupstream beforeoutput. They pass; they do not prove MSVC compilation, COMactivation, CameraFrameServer compatibility or applicationenumeration. Acceptance sequence remains: buildDLL, protectedinstallation/registration, enumerate/stream syntheticcamera in WindowsCamera and browser, uninstallcleanly; only then replace generator with JPEGbroker whose ACL and sessionidentity are explicitly validated. The mic endpoint remains separate outstandingdriverwork.
