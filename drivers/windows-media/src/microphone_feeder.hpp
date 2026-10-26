#pragma once
#include "media_bridge.hpp"
#include "microphone_endpoint.h"
#include <mutex>
namespace relevo {
// User-mode feeder: source is RVL1 22050 Hz mono PCM16; endpoint is 48000 mono
// PCM16. Linear interpolation preserves phase across arbitrary packet splits.
// Not called from a driver timer/DPC: the C ring above is the kernel boundary.
class MicrophoneFeeder {
 std::mutex mutex_;
 RelevoMicRing ring_{};
 bool connected_=false, previous_valid_=false;
 int16_t previous_=0;
 uint64_t input_index_=0,next_output_=0,last_us_=0;
 void reset() {
  relevo_mic_clear(&ring_);previous_valid_=false;input_index_=0;next_output_=0;
 }
 void sample(int16_t current) {
  if(!previous_valid_) {
   previous_valid_=true;previous_=current;input_index_=0;
   relevo_mic_push(&ring_,current);next_output_=22050;return;
  }
  ++input_index_;
  const uint64_t right=input_index_*48000;
  const uint64_t left=right-48000;
  while(next_output_<=right) {
   const int64_t delta=int64_t(current)-previous_;
   const int64_t value=int64_t(previous_)+delta*int64_t(next_output_-left)/48000;
   relevo_mic_push(&ring_,static_cast<int16_t>(value));next_output_+=22050;
  }
  previous_=current;
 }
public:
 static constexpr uint64_t StaleUS=250000;
 // p must have passed RVM1 decoding, but validate again before any mutation.
 void accept(const Packet& p,uint64_t now_us) {
  std::lock_guard<std::mutex> lock(mutex_);
  if(p.kind==MediaKind::Disconnect) {reset();connected_=false;return;}
  const auto& d=p.data;
  if(p.kind!=MediaKind::MicrophonePCM || d.size()<10 || d.size()>890 || d.size()%2 ||
     le32(d.data())!=22050 || le16(d.data()+4)!=1 || le16(d.data()+6)!=16)
   throw std::invalid_argument("microphone feeder format");
  if(!connected_ || now_us<last_us_ || now_us-last_us_>=StaleUS) reset();
  connected_=true;last_us_=now_us;
  for(size_t i=8;i<d.size();i+=2) {
   uint16_t bits=le16(d.data()+i);
   sample(static_cast<int16_t>(bits<32768 ? int32_t(bits):int32_t(bits)-65536));
  }
 }
 size_t read(int16_t* out,size_t frames,uint64_t now_us) {
  if(!out && frames) throw std::invalid_argument("microphone output");
  std::lock_guard<std::mutex> lock(mutex_);
  if(connected_ && (now_us<last_us_ || now_us-last_us_>=StaleUS)) {reset();connected_=false;}
  return relevo_mic_read(&ring_,out,frames);
 }
 size_t buffered() {std::lock_guard<std::mutex> lock(mutex_);return ring_.count;}
 uint64_t dropped() {std::lock_guard<std::mutex> lock(mutex_);return ring_.dropped;}
};
}
