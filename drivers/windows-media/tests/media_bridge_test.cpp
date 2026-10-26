#include "media_bridge.hpp"
#include <cassert>
#include <iostream>
using namespace relevo;
static std::vector<uint8_t> wire(uint16_t kind, size_t n) {
 std::vector<uint8_t> p(24+n);p[0]='R';p[1]='V';p[2]='M';p[3]='1';p[4]=1;p[6]=kind;
 for(int i=0;i<4;++i)p[12+i]=uint8_t(n>>(8*i));return p;
}
static bool rejects(std::vector<uint8_t> p){try{decode(p.data(),p.size());return false;}catch(const std::invalid_argument&){return true;}}
int main(){
 auto p=wire(2,890);p[24]=0x22;p[25]=0x56;p[28]=1;p[30]=16;
 auto packet=decode(p.data(),p.size());assert(packet.data.size()==890);
 p[28]=2;assert(rejects(p));p[28]=1;p.push_back(0);assert(rejects(p));
 assert(rejects(wire(3,2)));assert(rejects(wire(2,892)));assert(rejects(wire(9,0)));
 auto j=wire(1,4);j[24]=0xff;j[25]=0xd8;j[26]=0xff;j[27]=0xd9;
 assert(decode(j.data(),j.size()).kind==MediaKind::CameraJPEG);
 j[0]=0;assert(rejects(j));assert(rejects({}));
 Mailbox m;for(uint32_t i=0;i<10;i++){packet.sequence=i;m.push(packet);}
 Packet out;assert(m.pop(MediaKind::MicrophonePCM,out)&&out.sequence==4);
 Packet c{MediaKind::CameraJPEG,1,0,{}};m.push(c);c.sequence=2;m.push(c);
 assert(m.pop(MediaKind::CameraJPEG,out)&&out.sequence==2);assert(!m.pop(MediaKind::CameraJPEG,out));
 m.push(c);m.push({MediaKind::Disconnect,0,0,{}});
 assert(!m.pop(MediaKind::MicrophonePCM,out));assert(!m.pop(MediaKind::CameraJPEG,out));
 std::cout<<"media codec/queue tests passed\n";
}
