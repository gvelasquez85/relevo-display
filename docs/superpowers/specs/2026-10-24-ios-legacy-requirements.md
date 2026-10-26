> Actualización posterior del usuario, 24 de octubre de 2026: el dispositivo real tiene **iOS 9.3.5**, y ese mínimo es suficiente. Las referencias siguientes a iOS 7 y a soporte obligatorio de iOS 5.1.1 describen el supuesto anterior y quedan sustituidas para la siguiente implementación.

# Own iPad client — compatibility requirements

User requirement, 2026-10-24: retain support for the earliest iPads, including iOS5 or7. Interpret minimum target as iOS5.1.1, rather than7, to include original iPad. Compatibility is a required goal, not yet demonstrated.

Proposed architecture: versioned shared protocol and desktop servers, native lightweight Objective-C/UIKit legacy32bit client, separate modern build/client using features negotiated by capability. Basic display, touch and reconnect should remain available on legacy; modern-only optimizations cannot raise legacy floor.

First feasibility gate: produce a trivial32bit client, sign/install on actual iOS5.1.1/iOS7/iOS9 devices, establish local USB transport, display image and receive touch. Toolchain, signing and installation of new legacy binaries are unverified; modern TestFlight is not the legacy installation strategy. Do not promise same performance or fullfeature parity across devices. Maintain Yam compatibility while exploring own client.

Primary references consulted: https://developer.apple.com/xcode/system-requirements ; https://apps.apple.com/us/app/testflight/id899247664 ; https://support.apple.com/en-us/103597 .
