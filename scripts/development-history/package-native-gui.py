#!/usr/bin/env python3
"""Package the native GUI lab, excluding credentials and local diagnostics."""
from pathlib import Path
import hashlib, json, zipfile
root = Path(__file__).resolve().parent.parent
out = root / 'dist'
exe = out / 'Relevo-Pruebas-Windows-0.7.exe'
ipa = out / 'Relevo-iPad-0.6-unsigned.ipa'
guide = root / 'docs/native-gui-test-guide.md'
notices = root / 'THIRD_PARTY_NOTICES.txt'
files = [exe, ipa, guide, notices]
assert all(p.is_file() for p in files)
assert exe.read_bytes()[:2] == b'MZ'
with zipfile.ZipFile(ipa) as z:
    assert z.testzip() is None
hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in files}
bundle = out / 'Relevo-Pruebas-Windows-0.7.zip'
with zipfile.ZipFile(bundle, 'w', zipfile.ZIP_DEFLATED) as z:
    for p in files:
        z.write(p, 'LEEME.md' if p == guide else p.name)
    z.writestr('SHA256SUMS.txt', ''.join(f'{v}  {"LEEME.md" if k == guide.name else k}\n' for k, v in hashes.items()))
with zipfile.ZipFile(bundle) as z:
    assert z.testzip() is None
    for name in z.namelist():
        if name == 'SHA256SUMS.txt':
            continue
        source = guide if name == 'LEEME.md' else next(p for p in files if p.name == name)
        assert z.read(name) == source.read_bytes()
print(bundle)
print(hashlib.sha256(bundle.read_bytes()).hexdigest())
