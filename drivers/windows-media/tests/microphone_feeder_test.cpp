#include "microphone_feeder.hpp"
#include <cassert>
#include <iostream>
#include <thread>
using namespace relevo;
static Packet pcm(const std::vector<int16_t>& samples) {
 Packet p{MediaKind::MicrophonePCM,0,0,{0x22,0x56,0,0,1,0,16,0}};
 for(auto v:samples) {p.data.push_back(uint8_t(v));p.data.push_back(uint8_t(uint16_t(v)>>8));}
 return p;
}
int main() {
 RelevoMicEndpoint endpoint{};relevo_mic_open(&endpoint,7);
 std::vector<uint8_t> wire(26);wire[0]='R';wire[1]='V';wire[2]='M';wire[3]='2';
 wire[4]=1;wire[6]=1;wire[8]=7;wire[16]=1;wire[24]=0xff;wire[25]=0xff;
 assert(relevo_mic_ingest(&endpoint,wire.data(),wire.size())==1);
 assert(endpoint.ring.count==1);
 wire[8]=6;assert(!relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 wire[8]=7;wire[20]=1;assert(!relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 wire[20]=0;wire[6]=2;assert(!relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 wire.resize(24);wire[16]=0;assert(relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 assert(!endpoint.connected && endpoint.ring.count==0);
 assert(!relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 relevo_mic_open(&endpoint,8);assert(!relevo_mic_ingest(&endpoint,wire.data(),wire.size()));
 MicrophoneFeeder f;
 int16_t out[6000];
 assert(f.read(out,960,0)==0);for(int i=0;i<960;++i) assert(out[i]==0);
 f.accept(pcm({-32768,32767}),1);
 assert(f.read(out,4,1)==3);
 assert(out[0]==-32768 && out[1]>-3000 && out[1]<0 && out[2]>27000 && out[3]==0);
 // 2206 source samples span exactly 100 ms: 4801 output samples including t=0.
 MicrophoneFeeder a,b;
 std::vector<int16_t> source(2206);for(size_t i=0;i<source.size();++i)source[i]=int16_t(i*7-8000);
 for(size_t i=0;i<source.size();i+=441)
  a.accept(pcm(std::vector<int16_t>(source.begin()+i,source.begin()+std::min(i+441,source.size()))),2);
 for(auto v:source)b.accept(pcm({v}),2);
 int16_t other[6000];assert(a.read(out,6000,3)==4801);assert(b.read(other,6000,3)==4801);
 for(int i=0;i<6000;++i) assert(out[i]==other[i]);
 // Reject malformed input atomically, without erasing valid audio.
 f.accept(pcm({123,123}),4);auto count=f.buffered();auto bad=pcm({1});bad.data[4]=2;
 try{f.accept(bad,4);assert(false);}catch(const std::invalid_argument&){}
 assert(f.buffered()==count);
 // Oldest data is discarded at 120 ms, never accumulated indefinitely.
 for(int i=0;i<20;++i) f.accept(pcm(std::vector<int16_t>(441,int16_t(i))),5);
 assert(f.buffered()==RELEVO_MIC_CAPACITY && f.dropped()>0);
 assert(f.read(out,6000,250005)==0);for(auto v:out)assert(v==0);
 f.accept(pcm({111,111}),300000);assert(f.buffered()==3);
 f.accept({MediaKind::Disconnect,0,0,{}},300001);
 assert(f.read(out,10,300002)==0);for(int i=0;i<10;++i)assert(out[i]==0);
 f.accept(pcm({222,222}),300003);assert(f.read(out,3,300004)==3);for(int i=0;i<3;++i)assert(out[i]==222);
 // Portable user-mode reader and network producer may run concurrently.
 std::thread writer([&]{for(int i=0;i<1000;++i)f.accept(pcm({1,2,3}),400000);});
 std::thread reader([&]{int16_t small[48];for(int i=0;i<1000;++i)f.read(small,48,400000);});
 writer.join();reader.join();
 std::cout<<"microphone conversion, bounded queue, silence and concurrency tests passed\n";
}
