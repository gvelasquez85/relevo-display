#import "RVLANTransport.h"
#import <Security/Security.h>
#import <CommonCrypto/CommonDigest.h>
#import <sys/socket.h>
#import <poll.h>
#import <errno.h>
#import <fcntl.h>
#import <mach/mach_time.h>
#import <string.h>

static double RVNow(void) { mach_timebase_info_data_t info;mach_timebase_info(&info);return (double)mach_absolute_time()*info.numer/info.denom/1e9; }
typedef struct { int socket; short wanted; } RVSocketIO;
static OSStatus RVSocketRead(SSLConnectionRef connection,void *data,size_t *length) {
    RVSocketIO *io=(RVSocketIO *)connection;ssize_t n=recv(io->socket,data,*length,0);
    if(n>0){BOOL shortRead=(size_t)n<*length;*length=(size_t)n;if(shortRead)io->wanted|=POLLIN;return shortRead?errSSLWouldBlock:noErr;}
    *length=0;if(n==0)return errSSLClosedGraceful;io->wanted|=POLLIN;
    return errno==EAGAIN || errno==EWOULDBLOCK || errno==EINTR?errSSLWouldBlock:errSSLClosedAbort;
}
static OSStatus RVSocketWrite(SSLConnectionRef connection,const void *data,size_t *length) {
    RVSocketIO *io=(RVSocketIO *)connection;ssize_t n=send(io->socket,data,*length,0);
    if(n>0){BOOL shortWrite=(size_t)n<*length;*length=(size_t)n;if(shortWrite)io->wanted|=POLLOUT;return shortWrite?errSSLWouldBlock:noErr;}
    *length=0;io->wanted|=POLLOUT;return n<0 && (errno==EAGAIN || errno==EWOULDBLOCK || errno==EINTR)?errSSLWouldBlock:errSSLClosedAbort;
}
@implementation RVLANTransport {
    SSLContextRef _tls;NSLock *_lock;NSData *_pin;int _socket;RVSocketIO _io;
}
- (instancetype)initWithSocket:(int)socket pin:(NSData *)pin {
    self=[super init];if(!self)return nil;
    _socket=socket;_io.socket=socket;_pin=[pin copy];_lock=[NSLock new];
    int flags=fcntl(socket,F_GETFL,0);
    if(pin.length!=CC_SHA256_DIGEST_LENGTH || flags<0 || fcntl(socket,F_SETFL,flags|O_NONBLOCK)<0)return nil;
    _tls=SSLCreateContext(NULL,kSSLClientSide,kSSLStreamType);
    if(!_tls || SSLSetIOFuncs(_tls,RVSocketRead,RVSocketWrite)!=noErr || SSLSetConnection(_tls,&_io)!=noErr || SSLSetProtocolVersionMin(_tls,kTLSProtocol12)!=noErr || SSLSetProtocolVersionMax(_tls,kTLSProtocol12)!=noErr || SSLSetSessionOption(_tls,kSSLSessionOptionBreakOnServerAuth,true)!=noErr)return nil;
    return self;
}
- (BOOL)waitUntil:(double)deadline events:(short)events {
    double remaining=deadline-RVNow();if(remaining<=0)return NO;
    // SSLRead may need to write a TLS record and SSLWrite may need to read one.
    // The I/O callbacks record the actual blocked operation while holding _lock.
    struct pollfd p={_socket,events,0};int timeout=(int)MIN(100.0,MAX(1.0,remaining*1000.0));
    int result=poll(&p,1,timeout);
    // HUP can coexist with readable TLS records. Let SSL drain buffered data
    // and observe EOF itself; rejecting HUP here truncates a valid final frame.
    return (result>=0 || errno==EINTR) && !(p.revents&POLLNVAL) && RVNow()<deadline;
}
- (BOOL)authenticateToken:(NSData *)token {
    if(token.length!=16)return NO;
    double deadline=RVNow()+10;BOOL pinned=NO;
    while(RVNow()<deadline){
        _io.wanted=0;OSStatus status=SSLHandshake(_tls);short wanted=_io.wanted;
        if(status==errSSLServerAuthCompleted){
            SecTrustRef trust=NULL;BOOL valid=NO;
            if(SSLCopyPeerTrust(_tls,&trust)==noErr && trust && SecTrustGetCertificateCount(trust)>0){
                SecCertificateRef leaf=SecTrustGetCertificateAtIndex(trust,0);
                NSData *der=CFBridgingRelease(SecCertificateCopyData(leaf));uint8_t digest[CC_SHA256_DIGEST_LENGTH];
                if(der && der.length<=UINT32_MAX){CC_SHA256(der.bytes,(CC_LONG)der.length,digest);uint8_t difference=0;const uint8_t *expected=_pin.bytes;for(NSUInteger i=0;i<sizeof(digest);i++)difference|=digest[i]^expected[i];valid=difference==0;}
            }
            if(trust)CFRelease(trust);if(!valid)return NO;pinned=YES;continue;
        }
        if(status==noErr){if(!pinned)return NO;NSMutableData *auth=[NSMutableData dataWithBytes:"RVLP1" length:5];[auth appendData:token];uint8_t reply[3];return [self write:auth.bytes length:auth.length deadline:deadline] && [self read:reply length:3 deadline:deadline] && memcmp(reply,"OK1",3)==0;}
        if(status!=errSSLWouldBlock || ![self waitUntil:deadline events:wanted?wanted:POLLIN])return NO;
    }
    return NO;
}
- (BOOL)read:(void *)buffer length:(size_t)length { return [self read:buffer length:length deadline:RVNow()+15]; }
- (BOOL)read:(void *)buffer length:(size_t)length deadline:(double)deadline {
    uint8_t *p=buffer;
    while(length && RVNow()<deadline){size_t done=0;[_lock lock];_io.wanted=0;OSStatus status=SSLRead(_tls,p,length,&done);short wanted=_io.wanted;[_lock unlock];p+=done;length-=done;
        if(!length)return YES;if(status!=noErr && status!=errSSLWouldBlock)return NO;
        // Progress may leave plaintext inside SecureTransport even if the raw
        // socket is no longer readable. Consume it before polling again.
        if(done)continue;if(![self waitUntil:deadline events:wanted?wanted:POLLIN])return NO;}
    return length==0;
}
- (BOOL)write:(const void *)buffer length:(size_t)length { return [self write:buffer length:length deadline:RVNow()+3]; }
- (BOOL)write:(const void *)buffer length:(size_t)length deadline:(double)deadline {
    const uint8_t *p=buffer;
    while(length && RVNow()<deadline){size_t done=0;[_lock lock];_io.wanted=0;OSStatus status=SSLWrite(_tls,p,length,&done);short wanted=_io.wanted;[_lock unlock];p+=done;length-=done;
        if(!length)return YES;if(status!=noErr && status!=errSSLWouldBlock)return NO;
        if(done)continue;if(![self waitUntil:deadline events:wanted?wanted:POLLOUT])return NO;}
    return length==0;
}
- (void)dealloc {if(_tls)CFRelease(_tls);}
@end
