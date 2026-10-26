#!/usr/bin/env python3
"""Offline synthetic codec feasibility, never captures or connects to a device."""
from pathlib import Path
import json,shutil,subprocess,time
root=Path(__file__).resolve().parent.parent
out=root/'.tools/codec-comparison';out.mkdir(parents=True,exist_ok=True)
ffmpeg=shutil.which('ffmpeg')
if not ffmpeg:raise SystemExit('ffmpeg not found; benchmark optional')
results=[]
for codec,args,suffix in [('jpeg',['-c:v','mjpeg','-q:v','5','-f','mjpeg'],'mjpeg'),('h264',['-c:v','libx264','-profile:v','baseline','-level:v','3.1','-preset','veryfast','-tune','zerolatency','-b:v','4M','-maxrate','4M','-bufsize','4M','-bf','0','-g','30','-f','h264'],'h264')]:
 path=out/('synthetic.'+suffix)
 cmd=[ffmpeg,'-hide_banner','-loglevel','error','-y','-f','lavfi','-i','testsrc2=size=1024x768:rate=30','-t','3','-an','-pix_fmt','yuv420p','-threads','2']+args+[str(path)]
 start=time.monotonic();subprocess.run(cmd,check=True,capture_output=True);elapsed=time.monotonic()-start
 decode=[ffmpeg,'-hide_banner','-loglevel','error','-i',str(path),'-f','null','-']
 subprocess.run(decode,check=True,capture_output=True)
 results.append({'codec':codec,'bytes':path.stat().st_size,'encode_wall_seconds':elapsed,'command':cmd,'host_decode_success':True})
record={'scope':'Synthetic testsrc2,1024x768,30FPS,3sec,hostMac; not equal quality, not Windows/iPad runtime, not Yam framing or end-to-end latency','ffmpeg_version':subprocess.run([ffmpeg,'-version'],check=True,capture_output=True,text=True).stdout.splitlines()[0],'results':results}
p=root/'analysis/codec-offline-comparison.json';p.write_text(json.dumps(record,indent=2)+'\n');print(p)
