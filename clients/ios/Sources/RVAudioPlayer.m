#import "RVAudioPlayer.h"
#import <AudioToolbox/AudioToolbox.h>
#import <AVFoundation/AVFoundation.h>
#import <string.h>
@implementation RVAudioPlayer {
    NSLock *_lock;
    AudioQueueRef _queue;
    AudioQueueBufferRef _buffers[6];
    BOOL _busy[6], _started;
    NSUInteger _generation;
}
static void RVAudioDone(void *context, AudioQueueRef queue, AudioQueueBufferRef buffer) {
    RVAudioPlayer *player = (__bridge RVAudioPlayer *)context;
    [player->_lock lock];
    if (queue == player->_queue) for (int i=0;i<6;i++) if (player->_buffers[i]==buffer) player->_busy[i]=NO;
    [player->_lock unlock];
}
- (instancetype)init { if ((self=[super init])) _lock=[NSLock new]; return self; }
- (BOOL)start {
    [self stop];
    AVAudioSession *session=[AVAudioSession sharedInstance];
    if (![session setCategory:AVAudioSessionCategoryPlayback error:nil] || ![session setActive:YES error:nil]) return NO;
    AudioStreamBasicDescription format={0}; format.mSampleRate=22050; format.mFormatID=kAudioFormatLinearPCM;
    format.mFormatFlags=kLinearPCMFormatFlagIsSignedInteger|kLinearPCMFormatFlagIsPacked; format.mBytesPerPacket=2; format.mFramesPerPacket=1; format.mBytesPerFrame=2; format.mChannelsPerFrame=1; format.mBitsPerChannel=16;
    AudioQueueRef queue=NULL;
    if (AudioQueueNewOutput(&format,RVAudioDone,(__bridge void *)self,NULL,NULL,0,&queue)!=noErr) { [self stop]; return NO; }
    [_lock lock]; _queue=queue; BOOL ok=YES;
    for (int i=0;i<6;i++) if (AudioQueueAllocateBuffer(queue,882,&_buffers[i])!=noErr) ok=NO;
    [_lock unlock];
    if (!ok) { [self stop]; return NO; } return YES;
}
- (BOOL)consume:(NSData *)packet {
    if (packet.length<10 || packet.length>890 || ((packet.length-8)&1)) return NO;
    const uint8_t *p=packet.bytes;
    if (p[0]!=0x22 || p[1]!=0x56 || p[2] || p[3] || p[4]!=1 || p[5] || p[6]!=16 || p[7]) return NO;
    [_lock lock]; BOOL failed=NO;
    if (_queue) for (int i=0;i<6;i++) if (!_busy[i]) {
        AudioQueueBufferRef buffer=_buffers[i]; memcpy(buffer->mAudioData,p+8,packet.length-8); buffer->mAudioDataByteSize=(UInt32)packet.length-8;
        _busy[i]=YES;
        if (AudioQueueEnqueueBuffer(_queue,buffer,0,NULL)!=noErr) { _busy[i]=NO; failed=YES; }
        else if(!_started) { if(AudioQueueStart(_queue,NULL)==noErr) _started=YES; else failed=YES; }
        break;
    }
    NSUInteger generation=_generation;
    [_lock unlock];
    if(failed) dispatch_async(dispatch_get_main_queue(), ^{
        [self->_lock lock]; BOOL current=self->_generation==generation; [self->_lock unlock];
        if(current) [self stop];
    });
    return !failed;
}
- (void)stop {
    [_lock lock]; ++_generation; AudioQueueRef queue=_queue; _queue=NULL; _started=NO;
    for(int i=0;i<6;i++){_buffers[i]=NULL;_busy[i]=NO;} [_lock unlock];
    if(queue) {AudioQueueStop(queue,true);AudioQueueDispose(queue,true);}
    [[AVAudioSession sharedInstance] setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil];
}
- (void)dealloc { [self stop]; }
@end
