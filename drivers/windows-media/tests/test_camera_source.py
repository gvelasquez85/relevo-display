import hashlib, importlib.util, json, shutil, tempfile, unittest
from pathlib import Path
root=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('prepare_camera_source',root/'tools/prepare_camera_source.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class SourceIntegrity(unittest.TestCase):
 def test_generation_is_deterministic_and_complete(self):
  with tempfile.TemporaryDirectory() as t:
   a=module.prepare(Path(t)/'a');b=module.prepare(Path(t)/'b')
   self.assertEqual(sorted(p.name for p in a.iterdir()),sorted(p.name for p in b.iterdir()))
   for p in a.iterdir():self.assertEqual(p.read_bytes(),(b/p.name).read_bytes())
   self.assertIn(module.CLSID,(a/'VirtualCameraMediaSourceActivate.h').read_text())
   self.assertIn('IMFMediaSourceEx',(a/'SimpleMediaSource.h').read_text())
   self.assertIn('IMFMediaStream2',(a/'SimpleMediaStream.h').read_text())
   self.assertNotIn('m_spHWMediaSrc',(a/'VirtualCameraMediaSourceActivate.cpp').read_text())
 def test_camera_transport_is_opt_in_and_shutdown_releases_it(self):
  with tempfile.TemporaryDirectory() as t:
   a=module.prepare(Path(t)/'camera')
   metadata=json.loads((a/'ADAPTATION.json').read_text())
   self.assertTrue(metadata['camera_jpeg_ipc_implemented'])
   self.assertFalse(metadata['camera_pipe_enabled_by_default'])
   self.assertFalse(metadata['camera_pipe_windows_validated'])
   reader=(a/'camera_pipe_reader.hpp').read_text()
   self.assertIn('#ifdef RELEVO_ENABLE_EXPERIMENTAL_CAMERA_PIPE',reader)
   self.assertIn('CancelIoEx(pipe,&ov)',reader)
   self.assertIn('std::chrono::seconds(1)',reader)
   self.assertIn('SECURITY_IDENTIFICATION',reader)
   self.assertIn('m_spFrameGenerator.reset()', (a/'SimpleMediaStream.cpp').read_text())
   generator=(a/'SimpleFrameGenerator.cpp').read_text()
   self.assertIn('m_relevoCamera.render',generator)
   self.assertNotIn('int offset = curSysTimeInS',generator)
   self.assertIn('camera_pipe_reader.hpp',(a/'SimpleFrameGenerator.h').read_text())
 def test_modified_upstream_is_rejected_before_output(self):
  original=module.SOURCE
  try:
   with tempfile.TemporaryDirectory() as t:
    source=Path(t)/'vendor';shutil.copytree(original,source);module.SOURCE=source
    (source/'SimpleMediaSource.cpp').write_text('tampered')
    with self.assertRaisesRegex(ValueError,'hash mismatch'):module.prepare(Path(t)/'out')
    self.assertFalse((Path(t)/'out').exists())
  finally:module.SOURCE=original
if __name__=='__main__':unittest.main()
