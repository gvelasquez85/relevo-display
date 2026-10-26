"""Generate an original monitor glyph as Windows ICO (no external assets)."""
import struct
from pathlib import Path
parts=[]
for size in (16,32,48,64):
 pixels=bytearray()
 for y in reversed(range(size)):
  for x in range(size):
   xx,yy=x/size,y/size
   color=(20,31,48,255)
   if .12<=xx<.88 and .15<=yy<.70: color=(42,211,190,255)
   if .19<=xx<.81 and .22<=yy<.59: color=(23,48,70,255)
   if .44<=xx<.56 and .70<=yy<.81 or .30<=xx<.70 and .81<=yy<.87:color=(222,242,245,255)
   r,g,b,a=color;pixels+=bytes((b,g,r,a))
 header=struct.pack('<IiiHHIIiiII',40,size,size*2,1,32,0,len(pixels),0,0,0,0)
 parts.append((size,header+pixels+bytes(((size+31)//32)*4*size)))
 out=bytearray(struct.pack('<HHH',0,1,len(parts)));offset=6+16*len(parts)
 for size,data in parts:out+=struct.pack('<BBBBHHII',size,size,0,0,1,32,len(data),offset);offset+=len(data)
 for size,data in parts:out+=data
 Path('assets/relevo-monitor.ico').write_bytes(out)
