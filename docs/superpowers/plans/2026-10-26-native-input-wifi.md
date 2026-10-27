# Native input and Wi-Fi implementation plan

Goal: one Windows release offers the existing Yam USB path and our native iOS9.3.5 client with independent contacts, focus-driven/manual keyboard and authenticated Wi-Fi.

Architecture: RVL1 stays unchanged above USB or TLS. The iPad initiates Wi-Fi after scanning an ephemeral Windows QR; pin SHA256 of the leaf DER certificate and authenticate a random 128-bit token before RVL1. Windows binds one selected private IPv4; pairing expires and never enters logs/settings/files. Native iOS remains subject to Apple signing and verification.

- [x] iOS: retain stable contact IDs, cancel before layout/background changes, show keyboard only for a correlated single tap on an editable field or explicit keyboard mode. Add QR connection and pinned TLS1.2 while preserving loopback USB.
- [x] LAN: reject unauthenticated peers, bound deadlines/attempts, stop closes listening/authenticated sockets. Test pin/token/TLS/framing/cancellation.
- [x] GUI: select Yam/Relevo and USB/Wi-Fi in the same application, show local interface and ephemeral QR, retain main Stop and in-GUI input test.
- [x] Native runner: reuse capture/input, start display only after authenticated transport, release contacts before display removal; continuous main session and bounded GUI tests.
- [x] Delivery: race tests, portable/Windows vet, ARMv7 build, archive former release, one current exe/ZIP and updated IPA. Distinguish compilation/protocol evidence from physical device validation.

Verified locally: Go race suite and Windows cross-compilation; ARMv7 IPA built unsigned. Exact iOS TLS implementation interoperated with Go under macOS SecureTransport (pin/token rejection, fragmented512KiB transfers, orderly-close regression). Apple Vision decoded actual GUI QR pixels. Native iPad execution and Windows physical QA remain unverified; USB inventory was empty at delivery. Keyboard auto-focus currently covers native Edit/RichEdit controls, with explicit manual mode for custom/browser controls.
