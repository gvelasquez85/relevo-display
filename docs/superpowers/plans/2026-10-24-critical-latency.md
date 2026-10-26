# Critical latency: SIMD JPEG and direct capture

User reaffirmed latency critical beforeotherfeatures. Baseline reportedbyuser:1024x768,30requestedFPS,quality60,25–26senderFPS. No receiverpresentationFPS or endtoendlatency measured.

- NativeWindowsJPEG SIMD via pinnedofficiallibjpeg-turbo3.2.0 DLL, BGRA420FASTDCT; optionalfallbackGoJPEG.
- PersistenttopdownDIB, BitBlt exactsize, noGetDIBitscopy, noBGRAtoRGBA conversion onnativemode, noPatBlt whenfullframecovered.
- Preserve perframevirtualidentity/mode and pre/postUACguard; optimize namedtargetvalidation instead of queryingallphysicalregistry/modes.
- GUIreports actualselectedencoder. Add controlroundtripp95 diagnostic, notpresentationlatency.
- CleanupcachedprivateDLL onlyafterallsessionsstop, fullThirdPartylicense/IJG attribution; independentreview andportabletests/vet/crossbuild.
- Compare encodingonlyonhost atsame1024x768/quality60. Do not claimhostbenchmarksareWindows/iPadmeasurements. Actualusertrial remainsrequired.

NoH264,60FPS, touch orotherfeature expansioninthisrelease. Keepusercurrentresolution/FPS/quality so comparisonisolatescapture+encoderchanges.
