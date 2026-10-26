#import "RVMicrophone.h"
#import <AudioToolbox/AudioToolbox.h>
#import <AVFoundation/AVFoundation.h>
@implementation RVMicrophone {
    NSLock *_lock;
    AudioQueueRef _queue;
    NSUInteger _generation;
}
static void RVMicrophoneInput(void *context, AudioQueueRef queue, AudioQueueBufferRef buffer, const AudioTimeStamp *time, UInt32 packets, const AudioStreamPacketDescription *descriptions) {
    @autoreleasepool {
    RVMicrophone *mic=(__bridge RVMicrophone *)context;
    [mic->_lock lock];
    if(queue!=mic->_queue){[mic->_lock unlock];return;}
    UInt32 count=MIN(buffer->mAudioDataByteSize,882)&~1U;
    if(count) {
        NSMutableData *packet=[NSMutableData dataWithLength:8+count];
        uint8_t *p=packet.mutableBytes; p[0]=0x22;p[1]=0x56;p[4]=1;p[6]=16;
        memcpy(p+8,buffer->mAudioData,count);
        if(mic.packetReady)mic.packetReady(packet);
    }
    OSStatus status=AudioQueueEnqueueBuffer(queue,buffer,0,NULL);
    NSUInteger generation=mic->_generation;
    [mic->_lock unlock];
    if(status!=noErr) dispatch_async(dispatch_get_main_queue(), ^{
        if(mic->_generation==generation){[mic stop];if(mic.failed)mic.failed();}
    });
    }
}
- (instancetype)init {if((self=[super init]))_lock=[NSLock new];return self;}
- (BOOL)start {
    [self stop];
    AVAudioSession *session=[AVAudioSession sharedInstance];
    if(session.recordPermission!=AVAudioSessionRecordPermissionGranted)return NO;
    if(![session setCategory:AVAudioSessionCategoryPlayAndRecord withOptions:AVAudioSessionCategoryOptionDefaultToSpeaker error:nil] || ![session setActive:YES error:nil])return NO;
    AudioStreamBasicDescription format={0};format.mSampleRate=22050;format.mFormatID=kAudioFormatLinearPCM;
    format.mFormatFlags=kLinearPCMFormatFlagIsSignedInteger|kLinearPCMFormatFlagIsPacked;format.mBytesPerPacket=2;format.mFramesPerPacket=1;format.mBytesPerFrame=2;format.mChannelsPerFrame=1;format.mBitsPerChannel=16;
    AudioQueueRef queue=NULL;
    if(AudioQueueNewInput(&format,RVMicrophoneInput,(__bridge void *)self,NULL,NULL,0,&queue)!=noErr){[session setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil];return NO;}
    [_lock lock];_queue=queue;BOOL ok=YES;
    for(int i=0;i<3;i++){AudioQueueBufferRef buffer=NULL;if(AudioQueueAllocateBuffer(queue,882,&buffer)!=noErr || AudioQueueEnqueueBuffer(queue,buffer,0,NULL)!=noErr){ok=NO;break;}}
    [_lock unlock];
    if(!ok || AudioQueueStart(queue,NULL)!=noErr){[self stop];return NO;}return YES;
}
- (void)stop {
    [_lock lock];++_generation;AudioQueueRef queue=_queue;_queue=NULL;[_lock unlock];
    if(queue){AudioQueueStop(queue,true);AudioQueueDispose(queue,true);[[AVAudioSession sharedInstance] setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil];}
}
- (void)dealloc {[self stop];}
@end
