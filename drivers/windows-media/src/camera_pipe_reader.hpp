#pragma once
#ifdef _WIN32
#include <windows.h>
#include <atomic>
#include <chrono>
#include <mutex>
#include <thread>
#include <vector>
#include "camera_decode.hpp"
#include "camera_canvas.hpp"
#include "media_bridge.hpp"
namespace relevo {
// Disabled by default: FrameServer service identity and pipe ACLs require a
// Windows validation pass before the transport may be enabled for distribution.
// The GUI/host never needs to grant access to Everyone or expose a network port.
class CameraPipeReader {
 std::atomic<bool> stop_{false};std::thread worker_;std::mutex lock_;
 std::vector<uint8_t> pixels_;UINT width_=0,height_=0;
 std::chrono::steady_clock::time_point received_{};
 void clear(){std::lock_guard<std::mutex> g(lock_);pixels_.clear();width_=height_=0;}
 bool read_exact(HANDLE pipe,uint8_t* out,DWORD size){
  DWORD total=0;const auto until=std::chrono::steady_clock::now()+std::chrono::seconds(2);
  while(total<size && !stop_){
   OVERLAPPED ov{};ov.hEvent=CreateEventW(nullptr,TRUE,FALSE,nullptr);if(!ov.hEvent)return false;
   DWORD done=0;BOOL ok=ReadFile(pipe,out+total,size-total,&done,&ov);
   if(!ok && GetLastError()==ERROR_IO_PENDING){
    for(;;){DWORD wait=WaitForSingleObject(ov.hEvent,25);
     if(wait==WAIT_OBJECT_0){ok=GetOverlappedResult(pipe,&ov,&done,FALSE);break;}
     if(wait!=WAIT_TIMEOUT || stop_ || std::chrono::steady_clock::now()>=until){
      CancelIoEx(pipe,&ov);GetOverlappedResult(pipe,&ov,&done,TRUE);ok=FALSE;break;
     }
    }
   }
   CloseHandle(ov.hEvent);if(!ok || !done)return false;total+=done;
  }return total==size && !stop_;
 }
 void run(){
  HRESULT init=CoInitializeEx(nullptr,COINIT_MULTITHREADED);if(FAILED(init))return;
  struct COMScope {~COMScope(){CoUninitialize();}} comScope;
  while(!stop_){
   HANDLE pipe=CreateFileW(L"\\\\.\\pipe\\Relevo.Camera.v1",GENERIC_READ,0,nullptr,OPEN_EXISTING,
    FILE_FLAG_OVERLAPPED|SECURITY_SQOS_PRESENT|SECURITY_IDENTIFICATION,nullptr);
   if(pipe==INVALID_HANDLE_VALUE){for(int i=0;i<10 && !stop_;++i)Sleep(25);continue;}
   struct PipeScope {HANDLE h;~PipeScope(){CloseHandle(h);}} pipeScope{pipe};
   while(!stop_){
    uint8_t header[24];if(!read_exact(pipe,header,sizeof(header)))break;
    // Reject before allocating, including wrong protocol version/kind.
    const uint32_t len=le32(header+12);const uint16_t kind=le16(header+6);
    if(header[0]!='R'||header[1]!='V'||header[2]!='M'||header[3]!='1'||le16(header+4)!=1 ||
      (kind!=1 && kind!=3) || (kind==1 && (len<4 || len>512*1024)) || (kind==3 && len))break;
    std::vector<uint8_t> wire(header,header+24);wire.resize(24+len);
    if(len && !read_exact(pipe,wire.data()+24,len))break;
    try {
     auto packet=decode(wire.data(),wire.size());if(packet.kind==MediaKind::Disconnect){clear();continue;}
     std::vector<uint8_t> next;UINT w=0,h=0;
     if(FAILED(decode_camera_jpeg(packet.data,next,w,h))){clear();continue;}
     std::lock_guard<std::mutex> g(lock_);pixels_.swap(next);width_=w;height_=h;received_=std::chrono::steady_clock::now();
    } catch(...) {clear();break;}
   }
   clear();
  }
  clear();
 }
public:
 CameraPipeReader()=default;
 CameraPipeReader(const CameraPipeReader&)=delete;CameraPipeReader& operator=(const CameraPipeReader&)=delete;
 ~CameraPipeReader(){stop_=true;if(worker_.joinable())worker_.join();}
 bool render(uint8_t* out,size_t bytes,int pitch,unsigned width,unsigned height){
  std::lock_guard<std::mutex> g(lock_);
#ifdef RELEVO_ENABLE_EXPERIMENTAL_CAMERA_PIPE
  if(!worker_.joinable()) {
   try {worker_=std::thread([this]{try{run();}catch(...){clear();}});} catch(...) {/* black until retry */}
  }
#endif
  if(std::chrono::steady_clock::now()-received_>std::chrono::seconds(1))
   return camera_canvas({},0,0,out,bytes,pitch,width,height);
  return camera_canvas(pixels_,width_,height_,out,bytes,pitch,width,height);
 }
};
}
#endif
