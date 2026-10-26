# Relevo native protocol v1: first USB display path

Scope: build own native client toward Windows host while SDK downloads. Existing Yam-compatible app/host remains intact. This experimental path is separately named/packaged. First milestone is USB handshake, JPEG display and local-media controls; not virtualwebcam/full desktopaudio yet.

Transport: Apple/usbmux host connects device loopback TCP port52742. iPad listener binds127.0.0.1 only, no LAN listener. One active peer; background/Stop closes listener and peer. No personalidentifiers. User starts link explicitly.

Frame: 16byte big-endian header: bytes0..3 ASCII RVL1,4..5 type uint16,6..7 flags uint16 (must0),8..11 sequenceuint32,12..15 payloadlengthuint32. Validate magic/flags/type/limit BEFORE allocation. Known types 1 hello,2 start,3 displayJPEG,4 touchJSON,5 statusJSON,6 ping,7 pong,8 cameraJPEG reserved,9 audioPCM reserved,10 frameApplied. Max JSON4096bytes, JPEG4MiB, PCM65536bytes, ping/pong/frameApplied zero bytes. Unsupportedreservedtypes reject until implemented.

Handshake: iPad sends hello JSON {protocol:1,width:1024,height:768}; host sends start JSON {width:1024,height:768,fps:30}; iPad validates dimensions exactly1024x768 and fps1..60, sends status JSON {state:"ready"}. Host sends JPEG only after ready. Client software acknowledges application of image with type10 same sequence. Host allows one display frame outstanding; no FIFO. Acknowledgment means UIImage update performed, not physicalscreenpresentation. Touch message format reserved for future client implementation uses {phase:"down|move|up|cancel",x:0..1,y:0..1} relative to displayed image; no injection until Windows identity/transform/state handling separatelyvalidated. Host may countknownphase without loggingvalues.

Host experiment: own CLI cmd/relevo-native, pattern bydefault; optional desktop/extend reusescapture/virtualdriver policy. No receiverlicense dependency in ownprotocol. Includes bounded duration and explicitdisconnect cleanup. No changing installedYam. Camera/audio localproof retained in nativeapp; USB camera/audio transport remains futurework, reservedtype alone isnot implementation.

Implementation owners: iOSworker owns RVConnection.h/.m +DisplayViewController.m; hostworker owns internal/nativewire,internal/nativesession,cmd/relevo-native; parent owns project/plist/scripts/docs/packaging. Use identical protocol across languages. Tests include header vectors, shortwrites, oversized/unknownframes, hello/ready/JPEG/ack, cancellation. SDKcompile/sign/device validation still required before claiming installable app.

## Extension v0.4 (implemented only when explicitly negotiated)

Start JSON may include booleans touch,audio,camera; absence means false. Client ready STATUS includes those accepted booleans. Host MUST reject a requested capability not acknowledged; no silent downgrade.

Type4 TOUCH client→host: normalized finite x/y in[0,1], phase down/move/up/cancel. Only one finger/leftbutton in firstimplementation; no keyboard/rightclickclaim. Coordinates refer image-content rect excludingletterbox. Cancel/up always release. Windows target topology revalidated; UAC/securedesktop blocks injection; releases onstop/disconnect/background. Host flag -touch optin; unavailable forpattern.

Type8 CAMERA_JPEG client→host: max512KiB, JPEG validated max640×480 beforedecode. App explicitcamera permission/start, one latestframeoutgoing, dropifwriterbusy, targetmax10FPS, front/back onlyifavailable. Host preview isdistinct from virtualdevicepublication; not claimTeamscamerauntilvirtualsourceinstalled/tested.

Type9 AUDIO_PCM host→client: max65536bytes; first8bytes littleendian rate uint32(22050),channelsuint16(1),bitsuint16(16), thenPCM16LE evenlength, nominal20ms882bytes. FixedformatsolegacyiPadAudioQueueworks. Windows WASAPIloopbacksharedcapturescurrentoutput onlyon -audio flag, downmix/resampletolimitedqueue. Client visibleaudiooptin, max6buffers queued120ms, dropoverflow, releaseAudioQueue/sessiononstop/background. This isnot a Windowsoutputdevice driver ormicrophonecapture. Localtone remainsseparate.

All network writes mustserialize perconnection; socketwritesneverblockiOSmainthread. Camera pending slot and audiobuffers bounded, no unboundedFIFO. Oldv0.3peers keepdisplayonly start whenflagsabsent. Audio/camera/touch advertised only ifimplemented; neitherunknownpackettypesnorcapabilityfields implyautomaticauthorization.


## v0.5 negotiated input/microphone extension

Optional Start/ready booleans `multitouch`, `keyboard`, `microphone` defaultfalse; requested capabilities must be acknowledged. Frames rejected before ready or without respective capability. Types11and12 accept JSON up to8192bytes:11 `{contacts:[{id,phase,x,y}]}` changed-contactbatch (1..10items, stable IDs1..10 on nativeclient, normalized finite coordinates, down/move/up/cancel);12 `{kind,text?,code?}` with committedUTF8text<=4096bytes or explicitdown/up/presscontrolcodes. KeyA..KeyZ/Digit0..Digit9/Space/F1..F12 and modifiers/navigation supported. Unknownfields/missingcoordinates rejected; batches validated atomically. Textinput logs never contain values.

Type13 MicrophonePCM is device→host fixedformat identical to type9:22050Hz/mono/16bitLE header8bytes+2..882samplebytes (10..890total). Explicituserpermissionandbutton required; bounded3outgoingdevicepackets, hostdiagnostics or explicit6bufferwaveOutmonitor, no diskrecording. Monitoring is not a Windows systemmic. iOS0.5USBplayback and micactivation mutuallyexclusive; duplexfuture. Existingtype9 Windows→iPadaudio unchanged. Keyboard/touch/mic process whiledisplayACKpending. CLIcleanup releasesextendedinput beforedetachingdisplay. No OBS bridge dependency in v0.5 CLI.
