# Windows compatible receiver: parity and screen controls

Latest steering: pause owniOSclient/Xcode work; keep working RelevoWindows11 with installedYamreceiver on iPad2/iOS9.3.5. User supplied featurelist treated as reference data.

- [x] Produce matrix every suppliedfeature: existing, partial, missing, platform/hardware mismatch. Distinguish requestedFPS vsactualsender vsactualpresentation and driver60Hz.
- [x] Add GUI mirrorprimary vsvirtualextend, fourdirectionplacement, preserve settings, restoreonlyownedstate, negative/overflowtests.
- [x] KeepAwake onlywhilesession and cleanup oncancel/error; defaultsfalse and no persistent OSsettings.
- [x] Audit exact touchplist schema fromexistingbinaryevidence beforeWindowsinputimplementation. No injection ofunknowncoordinates.
- [x] Integrate/buildWindows testexe, portable/race/vet/crosscompile, independentreview. ActualWindowsruntime/usergesturecapture remainsneeded.

Owners workerwindows_display_controls wintray/winvirtual/desktop screenoptions; rootmain/docs/build/integration; kipp read-onlytouchschema. No need60FPSoptionuntilflowcanmeaningfullydeliver. UIpolish beyondfunctionalcontrolsremainslater.

Implementation/automatedchecks complete. Windowsruntime, livegesturecapture and touchinjection remain unverified; no claimphase1fluencyclosed.
