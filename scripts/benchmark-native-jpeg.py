#!/usr/bin/env python3
"""Isolated same-input/quality JPEG comparison on the development Mac only."""
from pathlib import Path
import json,os,subprocess,time
root=Path(__file__).resolve().parent.parent
work=root/'.tools/native-jpeg-bench';work.mkdir(parents=True,exist_ok=True)
prefix=Path('/opt/homebrew/opt/jpeg-turbo')
if not (prefix/'include/turbojpeg.h').is_file():raise SystemExit('Optional local libjpeg-turbo headers not installed')
pixels=bytearray(1024*768*4)
for y in range(768):
 for x in range(1024):
  i=(y*1024+x)*4;pixels[i:i+4]=bytes(((x*7+y*3)%256,(x*5+y*11)%256,(x*13+y*17)%256,255))
(work/'input.bgra').write_bytes(pixels)
(work/'native.c').write_text(r'''
#include <turbojpeg.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
int main(int argc,char**argv){
 unsigned char*src=malloc(1024*768*4);FILE*f=fopen(argv[1],"rb");if(!src||!f||fread(src,1,1024*768*4,f)!=1024*768*4)return 2;fclose(f);
 struct timespec a,b;clock_gettime(CLOCK_MONOTONIC,&a);unsigned long total=0;
 for(int i=0;i<90;i++){tjhandle h=tjInitCompress();unsigned char*out=NULL;unsigned long size=0;if(!h)return 3;
 if(tjCompress2(h,src,1024,4096,768,TJPF_BGRA,&out,&size,TJSAMP_420,60,TJFLAG_FASTDCT)!=0)return 4;
 total+=size;if(i==0){f=fopen(argv[2],"wb");fwrite(out,1,size,f);fclose(f);}tjFree(out);tjDestroy(h);}
 clock_gettime(CLOCK_MONOTONIC,&b);double elapsed=(b.tv_sec-a.tv_sec)+(b.tv_nsec-a.tv_nsec)/1e9;
 printf("{\"frames\":90,\"total_seconds\":%.6f,\"mean_ms\":%.4f,\"mean_bytes\":%lu}\n",elapsed,elapsed*1000/90,total/90);free(src);return 0;}
''')
(work/'go.go').write_text(r'''
package main
import("bytes";"encoding/json";"image";"image/jpeg";"os";"time")
func main(){src,e:=os.ReadFile(os.Args[1]);if e!=nil{panic(e)};img:=image.NewRGBA(image.Rect(0,0,1024,768));for i:=0;i<len(src);i+=4{img.Pix[i]=src[i+2];img.Pix[i+1]=src[i+1];img.Pix[i+2]=src[i];img.Pix[i+3]=255};started:=time.Now();total:=0;for i:=0;i<90;i++{var b bytes.Buffer;if e:=jpeg.Encode(&b,img,&jpeg.Options{Quality:60});e!=nil{panic(e)};total+=b.Len();if i==0{os.WriteFile(os.Args[2],b.Bytes(),0600)}};secs:=time.Since(started).Seconds();json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"frames":90,"total_seconds":secs,"mean_ms":secs*1000/90,"mean_bytes":total/90})}
''')
subprocess.run(['clang','-O2',str(work/'native.c'),'-I'+str(prefix/'include'),'-L'+str(prefix/'lib'),'-lturbojpeg','-o',str(work/'native')],check=True,capture_output=True)
subprocess.run([str(root/'.tools/go/bin/go'),'build','-o',str(work/'go'),str(work/'go.go')],check=True,capture_output=True)
res={}
for name in ['go','native']:
 rows=[]
 for i in range(3):
  p=subprocess.run([str(work/name),str(work/'input.bgra'),str(work/(name+'.jpg'))],check=True,capture_output=True,text=True);rows.append(json.loads(p.stdout))
 res[name]=rows
record={'scope':'Encoding only on Mac arm64, synthetic identical BGRA1024x768, quality60,YUV420,90frames x3. Native macOS library, NOT the bundled Windows DLL or end-to-end iPad latency. No device or screen captured.','platform':os.uname().sysname+' '+os.uname().machine,'native_library':str(prefix/'lib/libturbojpeg.dylib'),'runs':res}
(root/'analysis/native-jpeg-host-benchmark.json').write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps({k:[round(r['mean_ms'],2) for r in v] for k,v in res.items()}))
