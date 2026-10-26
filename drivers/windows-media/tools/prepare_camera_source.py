#!/usr/bin/env python3
"""Deterministic, hash-checked adaptation of Microsoft's actual MF COM source.
Does not install/register/compile a DLL or fetch dependencies.
"""
import argparse, hashlib, json
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / 'vendor/microsoft-camera'
FILES = ['pch.h','pch.cpp','SimpleFrameGenerator.h','SimpleFrameGenerator.cpp',
 'SimpleMediaSource.h','SimpleMediaSource.cpp','SimpleMediaStream.h','SimpleMediaStream.cpp',
 'VirtualCameraMediaSource.h','VirtualCameraMediaSourceActivate.h','VirtualCameraMediaSourceActivate.cpp',
 'VirtualCameraMediaSource.def','dllmain.cpp','winrtCommon.cpp']
CLSID = 'C869639B-87A0-49DC-951B-E53D34F059BA'
def prepare(target):
 manifest=json.loads((SOURCE/'UPSTREAM.json').read_text())
 for entry in manifest['files']:
  p=SOURCE/entry['path']
  if hashlib.sha256(p.read_bytes()).hexdigest()!=entry['sha256']:
   raise ValueError('upstream hash mismatch: '+entry['path'])
 target.mkdir(parents=True,exist_ok=True)
 for name in FILES:
  text=(SOURCE/name).read_text(encoding='utf-8-sig')
  text=text.replace('7B89B92E-FE71-42D0-8A41-E137D06EA184',CLSID)
  text=text.replace('0x7b89b92e, 0xfe71, 0x42d0, 0x8a, 0x41, 0xe1, 0x37, 0xd0, 0x6e, 0xa1, 0x84',
   '0xc869639b, 0x87a0, 0x49dc, 0x95, 0x1b, 0xe5, 0x3d, 0x34, 0xf0, 0x59, 0xba')
  if name=='pch.h':
   for header in ['EventHandler','HWMediaSource','HWMediaStream','AugmentedMediaSource','AugmentedMediaStream']:
    text=text.replace('#include "'+header+'.h"','')
  if name=='VirtualCameraMediaSourceActivate.h':
   text='\n'.join(line for line in text.splitlines() if 'm_spHWMediaSrc' not in line and 'm_spAugmentedMediaSrc' not in line)+'\n'
  if name=='VirtualCameraMediaSourceActivate.cpp':
   start=text.index('        bool bIsWrappingCamera = false;')
   end=text.index('        return S_OK;',start)
   text=text[:start]+'''        // Relevo software source never opens a physical Windows camera.
        if (!m_spSimpleMediaSrc)
        {
            m_spSimpleMediaSrc = winrt::make_self<winrt::WindowsSample::implementation::SimpleMediaSource>();
            RETURN_IF_FAILED(m_spSimpleMediaSrc->Initialize(this));
        }
        RETURN_IF_FAILED(m_spSimpleMediaSrc->QueryInterface(riid, ppv));
'''+text[end:]
   text=text.replace('        RETURN_IF_FAILED(m_spActivateAttributes->SetUINT32(MF_VIRTUALCAMERA_PROVIDE_ASSOCIATED_CAMERA_SOURCES, 1));','')
   text=text.replace('        m_spHWMediaSrc = nullptr;','').replace('        m_spAugmentedMediaSrc = nullptr;','')
   text=text.replace('IFACEMETHODIMP VirtualCameraMediaSourceActivate::ShutdownObject()\n    {\n        return S_OK;',
    'IFACEMETHODIMP VirtualCameraMediaSourceActivate::ShutdownObject()\n    {\n        if (m_spSimpleMediaSrc) { (void)m_spSimpleMediaSrc->Shutdown(); m_spSimpleMediaSrc = nullptr; }\n        return S_OK;')
  if name=='SimpleFrameGenerator.h':
   text=text.replace('#pragma once', '#pragma once\n#include "camera_pipe_reader.hpp"')
   text=text.replace('    UINT32 m_width = 0;', '    relevo::CameraPipeReader m_relevoCamera;\n    UINT32 m_width = 0;')
  if name=='SimpleFrameGenerator.cpp':
   start=text.index('    LONGLONG curSysTimeInS')
   end=text.index('    return S_OK;',start)
   text=text[:start]+'''    // Last frame wins; missing/stale/disconnected camera is black, never synthetic.
    if (!m_relevoCamera.render(pBuf, len, pitch, width, height)) return E_INVALIDARG;

'''+text[end:]
  if name=='SimpleMediaStream.cpp':
   text=text.replace('        m_bIsShutdown = true;', '        m_bIsShutdown = true;\n        m_spFrameGenerator.reset(); // cancel and join camera reader before releasing stream')
   text=text.replace('MF_MT_FRAME_RATE, 30, 1','MF_MT_FRAME_RATE, 10, 1').replace('* 8* 30','* 8* 10').replace('* 8* 30','* 8* 10').replace('SetSampleDuration(333333)','SetSampleDuration(1000000)')
  if name=='dllmain.cpp':
   text=text.replace('        *result = nullptr;', '        if (!result) return E_POINTER;\n        *result = nullptr;')
  (target/name).write_text(text)
 for name in ['camera_pipe_reader.hpp','camera_canvas.hpp','camera_decode.hpp','media_bridge.hpp']:
  (target/name).write_bytes((ROOT/'src'/name).read_bytes())
 (target/'ADAPTATION.json').write_text(json.dumps({'upstream':manifest['revision'],'source_clsid':CLSID,
  'mode':'camera-pipe-opt-in-black-default','camera_jpeg_ipc_implemented':True,
  'camera_pipe_enabled_by_default':False,'camera_pipe_windows_validated':False,'files':FILES},indent=2)+'\n')
 return target
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,required=True)
 print(prepare(parser.parse_args().output))
