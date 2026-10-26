#include "camera_canvas.hpp"
#include <cassert>
#include <vector>
int main(){
 std::vector<uint8_t> out(640*480*4,255), red(320*240*4,0);
 for(size_t i=0;i<red.size();i+=4)red[i+2]=255;
 assert(relevo::camera_canvas(red,320,240,out.data(),out.size(),640*4,640,480));
 assert(out[2]==255 && out[out.size()-2]==255);
 // Landscape image in portrait canvas: letterbox and preserve color.
 out.assign(240*320*4,255);
 assert(relevo::camera_canvas(red,320,240,out.data(),out.size(),240*4,240,320));
 assert(out[2]==0 && out[(70*240)*4+2]==255 && out.back()==0);
 assert(relevo::camera_canvas({},0,0,out.data(),out.size(),240*4,240,320));
 for(auto c:out)assert(c==0);
 assert(!relevo::camera_canvas(red,0,240,out.data(),out.size(),240*4,240,320));
 assert(!relevo::camera_canvas(red,320,240,out.data(),1,240*4,240,320));
 assert(!relevo::camera_canvas(red,320,240,out.data(),out.size(),-960,240,320));
 assert(!relevo::camera_canvas(red,320,240,nullptr,out.size(),960,240,320));
 assert(!relevo::camera_canvas(red,320,240,out.data(),out.size(),2600,650,1));
}
