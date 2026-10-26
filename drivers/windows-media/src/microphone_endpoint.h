#ifndef RELEVO_MICROPHONE_ENDPOINT_H
#define RELEVO_MICROPHONE_ENDPOINT_H
/* Kernel-safe storage contract. Caller serializes access (spin lock in WaveRT).
 * No STL, allocation, blocking, native pointers or OS handles on the wire. */
#include <stdint.h>
#include <stddef.h>
#define RELEVO_MIC_RATE 48000u
#define RELEVO_MIC_CHANNELS 1u
#define RELEVO_MIC_CAPACITY 5760u /* 120 ms; newest audio wins */
#define RELEVO_MIC_WRITE_MAX 960u /* 20 ms at endpoint format */
#define RELEVO_MIC_PROTOCOL 1u
/* Future METHOD_BUFFERED IOCTL payload. Explicit LE wire encoding required;
 * never cast an untrusted byte buffer to this struct. IOCTL numbers and device
 * ACL remain integration work; this header does not register a device. */
#define RELEVO_MIC_WIRE_HEADER 24u
/* bytes: magic RVM2[4], version u16, operation u16 (1 PCM,2 disconnect),
 * generation u64, frameCount u32, reserved u32=0, then frameCount s16LE. */
typedef struct RelevoMicRing {
 int16_t samples[RELEVO_MIC_CAPACITY];
 uint32_t first, count;
 uint64_t dropped;
} RelevoMicRing;
static inline void relevo_mic_clear(RelevoMicRing* r) { r->first=0; r->count=0; }
static inline void relevo_mic_push(RelevoMicRing* r, int16_t sample) {
 if(r->count==RELEVO_MIC_CAPACITY) {
  r->first=(r->first+1)%RELEVO_MIC_CAPACITY; --r->count; ++r->dropped;
 }
 r->samples[(r->first+r->count)%RELEVO_MIC_CAPACITY]=sample; ++r->count;
}
/* Returns genuine frames; always fills the requested capture buffer, with zero
 * on underrun. WaveRT must advance its sample clock even for silent frames. */
static inline size_t relevo_mic_read(RelevoMicRing* r,int16_t* out,size_t frames) {
 size_t got=0;
 for(size_t i=0;i<frames;++i) {
  if(r->count) {out[i]=r->samples[r->first];r->first=(r->first+1)%RELEVO_MIC_CAPACITY;--r->count;++got;}
  else out[i]=0;
 }
 return got;
}
typedef struct RelevoMicEndpoint {
 RelevoMicRing ring;
 uint64_t generation;
 int connected;
} RelevoMicEndpoint;
static inline uint32_t relevo_mic_u32(const uint8_t* p) {
 return (uint32_t)p[0]|((uint32_t)p[1]<<8)|((uint32_t)p[2]<<16)|((uint32_t)p[3]<<24);
}
/* Generation is allocated by trusted session-open code, not by incoming media.
 * Call under the same lock as ingest/read; a new session invalidates old writes. */
static inline void relevo_mic_open(RelevoMicEndpoint* e,uint64_t generation) {
 relevo_mic_clear(&e->ring);e->generation=generation;e->connected=1;
}
/* Future buffered-IOCTL decoder: validate every byte before touching state.
 * Returns 1 on acceptance, 0 on malformed/stale/closed payload. */
static inline int relevo_mic_ingest(RelevoMicEndpoint* e,const uint8_t* p,size_t size) {
 if(!p || size<RELEVO_MIC_WIRE_HEADER || p[0]!='R'||p[1]!='V'||p[2]!='M'||p[3]!='2' ||
    p[4]!=1 || p[5]!=0 || p[7]!=0 || relevo_mic_u32(p+20)!=0) return 0;
 uint64_t generation=(uint64_t)relevo_mic_u32(p+8)|((uint64_t)relevo_mic_u32(p+12)<<32);
 uint32_t count=relevo_mic_u32(p+16);
 if(!e->connected || generation!=e->generation || count>RELEVO_MIC_WRITE_MAX ||
    size!=RELEVO_MIC_WIRE_HEADER+(size_t)count*2) return 0;
 if(p[6]==2) {
  if(count) return 0;
  relevo_mic_clear(&e->ring);e->connected=0;return 1;
 }
 if(p[6]!=1 || !count) return 0;
 for(uint32_t i=0;i<count;++i) {
  uint32_t bits=(uint32_t)p[24+i*2]|((uint32_t)p[25+i*2]<<8);
  relevo_mic_push(&e->ring,(int16_t)(bits<32768?(int32_t)bits:(int32_t)bits-65536));
 }
 return 1;
}
#endif
