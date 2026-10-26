#!/bin/bash
# Experimental command-line build using an official SDK supplied by the owner.
# Produces an unsigned armv7 app for later signing, never installs it.
set -euo pipefail
cd -- "$(dirname -- "$0")"
relevo_sdk="${1:-}"
if [ -z "$relevo_sdk" ] || [ ! -f "$relevo_sdk/SDKSettings.plist" ] || [ ! -d "$relevo_sdk/System/Library/Frameworks/UIKit.framework" ]; then
  echo 'Usage: bash build-with-sdk.sh /absolute/path/to/iPhoneOS15.2.sdk' >&2
  echo 'An official armv7-capable iPhoneOS SDK is required; the current Mac has no selected iOS SDK.' >&2
  exit 2
fi
relevo_out="$PWD/build/unsigned/RelevoDisplay.app"
mkdir -p "$relevo_out"
xcrun clang -target armv7-apple-ios9.3.5 -isysroot "$relevo_sdk" \
  -fobjc-arc -fmodules -miphoneos-version-min=9.3.5 -O2 \
  Sources/main.m Sources/AppDelegate.m Sources/DisplayViewController.m Sources/RVConnection.m Sources/RVAudioPlayer.m Sources/RVMicrophone.m \
  -framework UIKit -framework Foundation -framework CoreGraphics -framework AVFoundation -framework ImageIO -framework AudioToolbox -framework CoreImage -framework CoreMedia -framework CoreVideo -lobjc \
  -o "$relevo_out/RelevoDisplay"
python3 - "$relevo_out" <<'PY'
from pathlib import Path
import plistlib,sys
with open('Info.plist','rb') as f:p=plistlib.load(f)
p.update(CFBundleExecutable='RelevoDisplay',CFBundleIdentifier='com.relevodisplay.client',MinimumOSVersion='9.3.5',CFBundleSupportedPlatforms=['iPhoneOS'],UIDeviceFamily=[2])
with open(Path(sys.argv[1])/'Info.plist','wb') as f:plistlib.dump(p,f)
PY
cp Assets/Icon76.png Assets/Icon152.png "$relevo_out/"
file "$relevo_out/RelevoDisplay"
echo "Unsigned app: $relevo_out"
echo 'Requires a valid provisioning profile and signing before installation on a normal iPad.'
