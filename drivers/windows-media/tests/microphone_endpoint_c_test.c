#include "microphone_endpoint.h"
#include <assert.h>
#include <string.h>
int main(void) {
 RelevoMicEndpoint e = {0};
 uint8_t packet[26] = {'R','V','M','2',1,0,1,0};
 int16_t output[3] = {1,1,1};
 relevo_mic_open(&e,7);
 packet[8]=7;packet[16]=1;packet[24]=0xff;packet[25]=0x7f;
 assert(relevo_mic_ingest(&e,packet,sizeof packet)==1);
 assert(relevo_mic_read(&e.ring,output,3)==1);
 assert(output[0]==32767 && output[1]==0 && output[2]==0);
 packet[8]=6;
 assert(relevo_mic_ingest(&e,packet,sizeof packet)==0);
 assert(e.ring.count==0);
 packet[8]=7;packet[6]=2;packet[16]=0;
 assert(relevo_mic_ingest(&e,packet,24)==1);
 packet[6]=1;packet[16]=1;
 assert(relevo_mic_ingest(&e,packet,sizeof packet)==0);
 relevo_mic_open(&e,8);
 assert(relevo_mic_ingest(&e,packet,sizeof packet)==0);
 return 0;
}
