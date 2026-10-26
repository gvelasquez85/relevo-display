#pragma once
#include <algorithm>
#include <cstdint>
#include <deque>
#include <mutex>
#include <stdexcept>
#include <vector>

namespace relevo {
// Portable codec/queue contract; no pointers or native struct padding on the wire.
enum class MediaKind : uint16_t { CameraJPEG = 1, MicrophonePCM = 2, Disconnect = 3 };
struct Packet { MediaKind kind; uint32_t sequence; uint64_t timestamp_us; std::vector<uint8_t> data; };
inline uint16_t le16(const uint8_t* p) { return uint16_t(p[0]) | uint16_t(p[1])<<8; }
inline uint32_t le32(const uint8_t* p) { return uint32_t(le16(p)) | uint32_t(le16(p+2))<<16; }
inline uint64_t le64(const uint8_t* p) { return uint64_t(le32(p)) | uint64_t(le32(p+4))<<32; }
inline Packet decode(const uint8_t* p, size_t n) {
 if (!p || n < 24 || p[0]!='R' || p[1]!='V' || p[2]!='M' || p[3]!='1' || le16(p+4)!=1)
  throw std::invalid_argument("media header");
 auto kind=static_cast<MediaKind>(le16(p+6)); auto size=le32(p+12);
 if (size != n-24) throw std::invalid_argument("media length");
 if (kind==MediaKind::CameraJPEG) {
  if(size<4 || size>512*1024 || p[24]!=0xff || p[25]!=0xd8 || p[n-2]!=0xff || p[n-1]!=0xd9)
   throw std::invalid_argument("camera payload");
 } else if(kind==MediaKind::MicrophonePCM) {
  if(size<10 || size>890 || size%2 || le32(p+24)!=22050 || le16(p+28)!=1 || le16(p+30)!=16)
   throw std::invalid_argument("microphone format");
 } else if(kind==MediaKind::Disconnect) {
  if(size) throw std::invalid_argument("disconnect payload");
 } else throw std::invalid_argument("media kind");
 return {kind,le32(p+8),le64(p+16),std::vector<uint8_t>(p+24,p+n)};
}
// Decoding occurs before queues. Camera dimensions must additionally be checked by WIC.
// A single JPEG and at most 120ms PCM are retained; disconnect clears both atomically.
class Mailbox {
 std::mutex lock_; std::deque<Packet> mic_; std::deque<Packet> camera_;
public:
 void push(Packet packet) {
  std::lock_guard<std::mutex> lock(lock_);
  if(packet.kind==MediaKind::Disconnect) {mic_.clear();camera_.clear();return;}
  auto& q=packet.kind==MediaKind::CameraJPEG?camera_:mic_;
  const size_t max=packet.kind==MediaKind::CameraJPEG?1:6;
  while(q.size()>=max) q.pop_front(); q.push_back(std::move(packet));
 }
 bool pop(MediaKind kind, Packet& out) {
  std::lock_guard<std::mutex> lock(lock_);
  auto& q=kind==MediaKind::CameraJPEG?camera_:mic_;
  if(q.empty()) return false; out=std::move(q.front());q.pop_front();return true;
 }
};
}
