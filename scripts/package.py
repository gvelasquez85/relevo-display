#!/usr/bin/env python3
"""Canonical Windows release: one GUI executable, no scripts or test bundles."""
from pathlib import Path
import hashlib, zipfile, shutil, datetime
root=Path(__file__).resolve().parent.parent
dist=root/'dist';exe=dist/'Relevo-Windows.exe';bundle=dist/'Relevo-Windows.zip'
assert exe.is_file() and exe.read_bytes()[:2]==b'MZ'
archive=dist/'archive'/datetime.datetime.now().strftime('%Y%m%d-pre-unified')
archive.mkdir(parents=True,exist_ok=True)
for p in list(dist.iterdir()):
 if p.is_file() and p.name not in {exe.name,bundle.name}:
  target=archive/p.name
  if target.exists():continue
  shutil.move(str(p),str(target))
with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED) as z:
 z.write(exe,exe.name)
 z.write(root/'THIRD_PARTY_NOTICES.txt','THIRD_PARTY_NOTICES.txt')
with zipfile.ZipFile(bundle) as z:
 assert z.testzip() is None
 assert z.read(exe.name)==exe.read_bytes()
print(bundle)
print('SHA256 '+hashlib.sha256(exe.read_bytes()).hexdigest())
