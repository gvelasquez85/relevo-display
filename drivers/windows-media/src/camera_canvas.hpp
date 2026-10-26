#pragma once
#include <algorithm>
#include <cstddef>
#include <cstdint>
#include <vector>
namespace relevo {
// Top-down BGRX canvas, preserve aspect ratio and black letterbox. No allocation
// follows untrusted dimensions; both source and destination are explicitly bounded.
inline bool camera_canvas(const std::vector<uint8_t>& image, unsigned sw, unsigned sh,
 uint8_t* output, size_t bytes, int pitch, unsigned dw, unsigned dh) {
 if (!output || !dw || !dh || dw>640 || dh>480 || pitch<static_cast<int>(dw*4) ||
     bytes/static_cast<unsigned>(pitch)<dh) return false;
 for(unsigned y=0;y<dh;++y) std::fill(output+y*pitch,output+y*pitch+dw*4,0);
 if(image.empty()) return true;
 if(!sw || !sh || sw>640 || sh>480 || image.size()!=size_t(sw)*sh*4) return false;
 unsigned w=dw,h=unsigned(uint64_t(sh)*dw/sw);
 if(h>dh){h=dh;w=unsigned(uint64_t(sw)*dh/sh);}
 if(!w || !h)return false;
 const unsigned ox=(dw-w)/2,oy=(dh-h)/2;
 for(unsigned y=0;y<h;++y)for(unsigned x=0;x<w;++x){
  const auto* src=image.data()+(size_t(y)*sh/h*sw+size_t(x)*sw/w)*4;
  auto* dst=output+(y+oy)*pitch+(x+ox)*4;
  std::copy(src,src+3,dst);dst[3]=0;
 }
 return true;
}
}
