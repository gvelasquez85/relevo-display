# Optional Windows SIMD JPEG encoder

`EncodeBGRA` accepts a top-down four-byte BGRA frame directly, avoiding the Go
BGRA-to-RGBA loop and pure-Go JPEG implementation. It preserves JPEG transport
compatibility with the current iPad receiver. This is an encoder implementation
change, not a video codec/protocol change.

Windows amd64 uses official libjpeg-turbo 3.2.0. Other platforms return
`ErrUnavailable`; callers must preserve their existing JPEG fallback. Errors
also fall back safely when the native DLL cannot be loaded. A process loads the
embedded DLL once from a unique directory under the current user's cache, after
SHA256 verification. It rejects reparse ancestors and opens a read handle without
write/delete sharing before final verification and loading. Before enabling native capture, the process encodes and decodes a deterministic
8×8 red frame to validate dimensions, BGRA order and the call ABI. A failed
self-test disables native capture and preserves the Go fallback. Dependency lookup is
restricted to the DLL directory and Windows System32. Only KERNEL32.dll and
msvcrt.dll are imported by the pinned binary.

The DLL remains loaded and its file read-locked until `Shutdown`, called at final
application exit after capture workers stop. Shutdown waits for active encodes,
unloads the DLL, closes its file handle, and removes precisely its owned DLL and
empty private directory. It does not initialize an unused encoder; calls after
shutdown return `ErrUnavailable`. Each encode
owns and frees its compressor handle and output allocation. Input is borrowed
synchronously, and output is copied once into Go ownership. The Windows x64 ABI
uses 64-bit pointers but 32-bit `unsigned long` output lengths. The implementation
uses BGRA=8, YUV420=2, and FASTDCT=2048 from the pinned `turbojpeg.h`.

Reproduce the dependency with `python3 scripts/fetch-turbojpeg.py`. This downloads
only from the official GitHub release, verifies the release API's pinned asset
SHA256, extracts without executing the installer, then verifies the DLL SHA256.
Provenance and complete upstream licensing are in `assets/`. Distribution must
include both license notices and this attribution:

> This software is based in part on the work of the Independent JPEG Group.

Validation on macOS: portable input validation tests and Windows amd64
cross-compilation/vet pass. `TestNativeJPEG` must run on Windows to validate the
actual load, call ABI, JPEG dimensions, and BGRA channel order. No Windows runtime
or throughput result is claimed from cross-compilation alone.
