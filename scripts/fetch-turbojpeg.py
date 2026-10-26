#!/usr/bin/env python3
"""Reproduce the embedded Windows x64 encoder from its official pinned release.
Never executes the installer. Requires 7z/7zz solely for NSIS archive extraction.
"""
import hashlib
import json
import pathlib
import shutil
import subprocess
import tempfile
import urllib.request

VERSION = '3.2.0'
URL = 'https://github.com/libjpeg-turbo/libjpeg-turbo/releases/download/3.2.0/libjpeg-turbo-3.2.0-gcc-x64.exe'
INSTALLER_SHA = '5a71ea596c573ea3b44c8e7b5e78613d3a28dc9490dc714e7222c9f63f55e454'
DLL_SHA = '4fd997c7eec636d3c48a9fc604b6cb573da030f2f6382d4ed07397c7d7f4e50e'
ROOT = pathlib.Path(__file__).resolve().parents[1]
DEST = ROOT / 'internal/jpegfast/assets'

def check(data, expected):
    actual = hashlib.sha256(data).hexdigest()
    if actual != expected:
        raise SystemExit(f'SHA256 mismatch: expected {expected}, got {actual}')

extractor = shutil.which('7zz') or shutil.which('7z')
if not extractor:
    raise SystemExit('Install 7zz/7z to extract the official NSIS archive; do not run it.')
with tempfile.TemporaryDirectory(prefix='relevo-turbojpeg-') as temp:
    temp = pathlib.Path(temp)
    installer = temp / 'installer.exe'
    with urllib.request.urlopen(URL, timeout=60) as response:
        data = response.read(16 * 1024 * 1024)
    check(data, INSTALLER_SHA)
    installer.write_bytes(data)
    subprocess.run([extractor, 'x', '-y', '-o' + str(temp / 'files'), str(installer)], check=True, stdout=subprocess.DEVNULL)
    source = temp / 'files'
    dll = (source / 'bin/libturbojpeg.dll').read_bytes()
    check(dll, DLL_SHA)
    DEST.mkdir(parents=True, exist_ok=True)
    (DEST / 'libturbojpeg.dll').write_bytes(dll)
    for filename in ['LICENSE.md', 'README.ijg']:
        shutil.copyfile(source / 'doc' / filename, DEST / filename)
    print(f'Verified libjpeg-turbo {VERSION}, Windows amd64: {DLL_SHA}')
