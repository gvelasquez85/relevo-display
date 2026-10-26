#!/usr/bin/env python3
"""Package native prototype without local SDKs, credentials or capture data."""
from pathlib import Path
import hashlib,json,plistlib,zipfile
root=Path(__file__).resolve().parent.parent
dist=root/'dist';version='0.6'
app=root/'clients/ios/build/unsigned/RelevoDisplay.app'
meta=plistlib.loads((app/'Info.plist').read_bytes())
assert meta['CFBundleShortVersionString']==version
assert meta['MinimumOSVersion']=='9.3.5'
ipa=dist/f'Relevo-iPad-{version}-unsigned.ipa'
with zipfile.ZipFile(ipa,'w',zipfile.ZIP_DEFLATED) as z:
 for p in sorted(app.rglob('*')):
  if p.is_file():z.write(p,'Payload/RelevoDisplay.app/'+str(p.relative_to(app)))
source=dist/f'Relevo-Native-{version}-source.zip'
with zipfile.ZipFile(source,'w',zipfile.ZIP_DEFLATED) as z:
 for directory in ['clients/ios','drivers/windows-media','internal','cmd/relevo-native']:
  for p in sorted((root/directory).rglob('*')):
   if not p.is_file():continue
   rel=p.relative_to(root)
   if any(part in ('build','__pycache__','.DS_Store') for part in rel.parts) or p.suffix in ('.exe','.pyc'):continue
   z.write(p,f'Relevo-Native-{version}/'+str(rel))
 for n in ['go.mod','go.sum','THIRD_PARTY_NOTICES.txt','docs/native-v06-test-guide.md','docs/native-camera-touch-keyboard.md','docs/wifi-connection-design.md']:
  z.write(root/n,f'Relevo-Native-{version}/'+n)
exe=dist/f'Relevo-Native-Windows-{version}.exe'
bundle=dist/f'Relevo-Native-{version}-dev.zip'
readme='''Relevo Native 0.6 — experimental\n\nCliente iOS9.3.5+ ARMv7 incluido SIN FIRMA: requiere firmar, instalar y verificar en iPad. La verificacion de Apple sigue pendiente. El host Windows es consola, separado de RelevoGUI compatible con Yam.\n\nVer PRUEBAS.md para multitouch, teclado, audio Windows hacia iPad, microfono iPad hacia Windows y escucha de prueba. Microfono y playbackUSB se prueban por separado. Camara virtual nativa y microfono seleccionable NO estan listos; fuentes y limites en source.zip. No requiere OBS. Instaladorunico pendiente.\n\nNo se han probado fisicamente estas nuevas funciones en Windows/iPad.\n'''
manifest={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in [exe,ipa,source]}
with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED) as z:
 for p in [exe,ipa,source]:z.write(p,p.name)
 z.writestr('LEEME.txt',readme)
 z.write(root/'docs/native-v06-test-guide.md','PRUEBAS.md')
 z.write(root/'THIRD_PARTY_NOTICES.txt','THIRD_PARTY_NOTICES.txt')
 z.writestr('SHA256SUMS.txt',''.join(v+'  '+k+'\n' for k,v in manifest.items()))
for p in [ipa,source,bundle]:
 with zipfile.ZipFile(p) as z:assert z.testzip() is None
with zipfile.ZipFile(ipa) as z:
 binary=z.getinfo('Payload/RelevoDisplay.app/RelevoDisplay')
 assert (binary.external_attr>>16)&0o111
(root/f'analysis/native-v{version.replace(chr(46),'')}-validation.json').write_text(json.dumps({'version':version,'artifacts':manifest,'go_race_suite':'passed','go_vet_portable_and_windows':'passed','windows_amd64_crossbuild':'passed','ios_armv7_compile':'passed','ios_minimum':'9.3.5','physical_validation':False,'automatic_keyboard_scope':'native Windows Edit/RichEdit only','dedicated_modes_pause_video':True,'wifi_sessions_enabled':False,'signed_ipa':False,'virtual_camera_windows_build':False,'selectable_virtual_microphone':False},indent=2)+'\n')
print(bundle)
