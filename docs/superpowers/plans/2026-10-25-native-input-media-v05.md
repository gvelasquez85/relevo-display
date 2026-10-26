# Native input and media v0.5 implementation plan

Goal: advance independent native iOS9.3.5 client and Windows host with multitouch, keyboard and microphone while full Xcode extraction continues.
Architecture: RVL1 optional negotiated capabilities extend v0.4; missing flags remain false. Types11 multiTouch JSON,12 keyboard JSON,13 microphone PCM. Keep display/audio/camera bounded and explicitly user activated. Input cleanup on disconnect before display detachment.
Tech: Objective-C/UIKit/AVFoundation/AudioQueue; Go Windows APIs; Media Foundation camera source evaluated separately.
Spec: docs/native-camera-touch-keyboard.md.

- [x] Nativewire types11/12 JSON <=8192 (keyboard4096UTF8 text plus envelope); type13 PCM same22050mono16/<=890 bytes. Contact payload {contacts:[{id,phase,x,y}]} complete changed-contact batches; normalized image coordinates; nonzero IDs <=10 active. Keyboard payload {kind:text|down|up|press,text?,code?}. Capability keys multitouch,keyboard,microphone all optional booleans and must be requested/acknowledged. No automatic forwarding from old single touch.
- [x] iOS owner: clients/ios only, integrate real multitouch events + native keyboard confirmed text/control keys + explicit microphone AudioQueue capture, bounded outgoing queue, same PCM format. Separate capability callback; preserve current v0.4 compatibility and audio playback. Update project/build entries and Info microphone usage. No bypass of system consent.
- [x] Windows input owner: internal/wininput only, new extended controller uses remoteinput validation, real InjectTouchInput and SendInput Unicode/control events. Export NewExtended(target,backend), NativeExtendedBackend(), HandleContacts([]remoteinput.Contact), HandleKey(remoteinput.Key), Close(). Adapt API if needed with parent coordination; no edits to session/CLI.
- [x] Parent: nativewire/nativesession/CLI capabilities, limits, integration and mic diagnostics receive endpoint. Tests before behavior changes, real net.Pipe sessions; unnegotiated frames rejected; callbacks and disconnect cleanup tested. No mic packets/images/text stored in logs.
- [ ] Media owner: windows device camera/microphone research and independently reviewable implementation under drivers/windows-media only. Prefer official Microsoft base, document licenses, prerequisites and honest compile limits. No OBS dependency; no installation of unsigned kernel driver/security changes. Distinguish preview/loopback from system virtual devices. Deliver integration API/build instructions and identify runtime gaps.
- [x] Verify Go suite -race, Windows crossbuild/vet, Objective-C SDK compile, package source+experimental binaries with explicit limitations. Physical Windows/iPad validation remains pending device trust.

No paid membership, certificate revocation, external publication or changes to iPad security authorized. Keep legacy working Yam-compatible executable unchanged.

Media partial: actual vendor COM synthetic baseline/buildcontrol/queues delivered and portabletested, but Windowsbuild, iPadJPEGbrokerfeed, virtualmicrophoneendpoint and unifiedinstaller remain pending. Native0.5 devZIP delivered with unsignedIPA and Windowshost. No hardwareclaims.
