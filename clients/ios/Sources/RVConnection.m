#import "RVConnection.h"
#import <sys/socket.h>
#import <netinet/in.h>
#import <unistd.h>
#import <errno.h>
#import <string.h>
#import <ImageIO/ImageIO.h>
#import <fcntl.h>
#import <poll.h>
#import <math.h>

@interface RVConnection ()
@property(atomic) NSUInteger generation;
@property(atomic, readwrite) BOOL touchAccepted, cameraAccepted, audioAccepted, multitouchAccepted, keyboardAccepted, microphoneAccepted, pointerAccepted, inputFocusAccepted, inputModesAccepted;
@property(atomic, readwrite) uint32_t latestTouchSequence;
@end
@implementation RVConnection {
    dispatch_queue_t _reader, _writer;
    NSUInteger _pendingControl; BOOL _pendingCamera; NSUInteger _pendingMicrophone;
    NSLock *_socketLock, *_writeLock;
    int _listener, _peer;
}
static uint16_t RVRead16(const uint8_t *p) { return ((uint16_t)p[0] << 8) | p[1]; }
static uint32_t RVRead32(const uint8_t *p) { return ((uint32_t)RVRead16(p) << 16) | RVRead16(p + 2); }
static void RVWrite16(uint8_t *p, uint16_t v) { p[0] = v >> 8; p[1] = v & 255; }
static void RVWrite32(uint8_t *p, uint32_t v) { RVWrite16(p, v >> 16); RVWrite16(p + 2, v & 65535); }
static BOOL RVReadAll(int fd, void *buffer, size_t length) {
    uint8_t *p = buffer;
    while (length) { ssize_t n = recv(fd, p, length, 0); if (n < 0 && errno == EINTR) continue; if (n <= 0) return NO; p += n; length -= (size_t)n; }
    return YES;
}
static BOOL RVWriteAll(int fd, const void *buffer, size_t length) {
    const uint8_t *p = buffer;
    while (length) { ssize_t n = send(fd, p, length, 0); if (n < 0 && errno == EINTR) continue; if (n <= 0) return NO; p += n; length -= (size_t)n; }
    return YES;
}
- (instancetype)init {
    self = [super init];
    if (self) { _writer=dispatch_queue_create("com.relevodisplay.usb.write", DISPATCH_QUEUE_SERIAL); _listener = _peer = -1; _socketLock = [NSLock new]; _writeLock = [NSLock new]; _reader = dispatch_queue_create("com.relevodisplay.usb.read", DISPATCH_QUEUE_SERIAL); }
    return self;
}
- (void)status:(NSString *)text generation:(NSUInteger)generation {
    dispatch_async(dispatch_get_main_queue(), ^{ if (self.generation == generation && self.statusChanged) self.statusChanged(text); });
}
- (void)stop {
    [_socketLock lock]; ++self.generation; [_socketLock unlock];
    self.touchAccepted=NO; self.cameraAccepted=NO; self.audioAccepted=NO; self.multitouchAccepted=NO; self.keyboardAccepted=NO; self.microphoneAccepted=NO; self.pointerAccepted=NO; self.inputFocusAccepted=NO; self.inputModesAccepted=NO; [self invalidateInputFocus];
    if(self.extendedCapabilitiesChanged) self.extendedCapabilitiesChanged(NO,NO,NO);
    if (self.capabilitiesChanged) self.capabilitiesChanged(NO,NO,NO);
    [_socketLock lock];
    // Only shutdown here. The reader owns close, preventing descriptor reuse races.
    if (_listener >= 0) shutdown(_listener, SHUT_RDWR);
    if (_peer >= 0) shutdown(_peer, SHUT_RDWR);
    [_socketLock unlock];
}
- (BOOL)send:(uint16_t)type sequence:(uint32_t)sequence data:(NSData *)data fd:(int)fd generation:(NSUInteger)generation {
    [_writeLock lock];
    uint8_t header[16] = {'R','V','L','1'};
    RVWrite16(header + 4, type); RVWrite16(header + 6, 0); RVWrite32(header + 8, sequence); RVWrite32(header + 12, (uint32_t)data.length);
    BOOL ok = self.generation == generation && RVWriteAll(fd, header, sizeof(header)) && (!data.length || RVWriteAll(fd, data.bytes, data.length));
    [_writeLock unlock]; return ok;
}
- (BOOL)sendJSON:(NSDictionary *)json type:(uint16_t)type fd:(int)fd generation:(NSUInteger)generation {
    NSData *data = [NSJSONSerialization dataWithJSONObject:json options:0 error:nil];
    return data && [self send:type sequence:0 data:data fd:fd generation:generation];
}
// Bounded asynchronous submissions; control overflow closes the peer so host releases buttons.
- (void)enqueue:(NSData *)data type:(uint16_t)type { [self enqueue:data type:type sequence:0]; }
- (void)enqueue:(NSData *)data type:(uint16_t)type sequence:(uint32_t)sequence {
    NSUInteger generation=self.generation;
    [_socketLock lock];
    BOOL camera=type==8, microphone=type==13;
    if (_peer<0 || (camera && _pendingCamera) || (microphone && _pendingMicrophone>=3)) { [_socketLock unlock]; return; }
    if (!camera && !microphone && _pendingControl>=16) { if(_peer>=0) shutdown(_peer,SHUT_RDWR); [_socketLock unlock]; return; }
    if(camera) _pendingCamera=YES; else if(microphone) _pendingMicrophone++; else _pendingControl++;
    int fd=_peer; [_socketLock unlock];
    dispatch_async(_writer, ^{
        BOOL ok=[self send:type sequence:sequence data:data fd:fd generation:generation];
        [self->_socketLock lock];
        if(camera) self->_pendingCamera=NO; else if(microphone) self->_pendingMicrophone--; else self->_pendingControl--;
        if(!ok && self.generation==generation && self->_peer==fd) shutdown(fd,SHUT_RDWR);
        [self->_socketLock unlock];
    });
}
- (void)sendTouch:(NSString *)phase x:(double)x y:(double)y {
    if (!self.touchAccepted || !isfinite(x) || !isfinite(y) || x<0 || x>1 || y<0 || y>1) return;
    if (![@[@"down",@"move",@"up",@"cancel"] containsObject:phase]) return;
    if([phase isEqualToString:@"move"]) { [_socketLock lock]; BOOL crowded=_pendingControl>=8; [_socketLock unlock]; if(crowded)return; }
    NSData *data=[NSJSONSerialization dataWithJSONObject:@{@"phase":phase,@"x":@(x),@"y":@(y)} options:0 error:nil];
    if([phase isEqualToString:@"down"])[self invalidateInputFocus];
    [self enqueue:data type:4 sequence:self.latestTouchSequence];
}
- (void)sendContacts:(NSArray *)contacts {
    if(!self.multitouchAccepted || !contacts.count || contacts.count>10)return;
    NSData *data=[NSJSONSerialization dataWithJSONObject:@{@"contacts":contacts} options:0 error:nil];
    BOOL moves=YES;for(NSDictionary *contact in contacts)if(![contact[@"phase"] isEqualToString:@"move"])moves=NO;
    if(moves){[_socketLock lock];BOOL crowded=_pendingControl>=8;[_socketLock unlock];if(crowded)return;}
    for(NSDictionary *contact in contacts)if([contact[@"phase"] isEqualToString:@"down"]){[self invalidateInputFocus];break;}
    if(data.length<=8192) [self enqueue:data type:11 sequence:self.latestTouchSequence];
}
- (void)invalidateInputFocus { uint32_t next=self.latestTouchSequence+1;self.latestTouchSequence=next?next:1; }
- (void)sendInputMode:(NSString *)mode {
    if(!self.inputModesAccepted || ![@[@"screen",@"keyboard",@"pointer"] containsObject:mode])return;
    NSData *data=[NSJSONSerialization dataWithJSONObject:@{@"mode":mode} options:0 error:nil];[self enqueue:data type:16];
}
- (void)sendPointer:(NSDictionary *)pointer {
    if(!self.pointerAccepted)return;
    NSData *data=[NSJSONSerialization dataWithJSONObject:pointer options:0 error:nil];
    if(data.length && data.length<=4096)[self enqueue:data type:15];
}
- (void)sendKey:(NSDictionary *)key {
    if(!self.keyboardAccepted)return;
    NSData *data=[NSJSONSerialization dataWithJSONObject:key options:0 error:nil];
    if(data.length && data.length<=8192) [self enqueue:data type:12];
}
- (void)sendMicrophone:(NSData *)packet {
    if(self.microphoneAccepted && packet.length>=10 && packet.length<=890 && !((packet.length-8)&1)) [self enqueue:packet type:13];
}
- (void)sendCamera:(NSData *)jpeg { if(self.cameraAccepted && jpeg.length && jpeg.length<=512*1024) [self enqueue:jpeg type:8]; }
- (void)start {
    [self stop];
    NSUInteger generation = self.generation;
    [self status:@"Preparando enlace USB…" generation:generation];
    dispatch_async(_reader, ^{ [self run:generation]; });
}
- (void)run:(NSUInteger)generation {
    if (self.generation != generation) return;
    int listener = socket(AF_INET, SOCK_STREAM, 0), peer = -1;
    NSString *result = @"Enlace USB desconectado. Pulsa Iniciar para volver a conectar.";
    if (listener < 0) { [self status:@"No se pudo crear el enlace USB." generation:generation]; return; }
    [_socketLock lock];
    if (self.generation != generation) { [_socketLock unlock]; close(listener); return; }
    _listener = listener;
    [_socketLock unlock];
    int enabled = 1;
    int listenerFlags = fcntl(listener, F_GETFL, 0);
    BOOL nonblocking = listenerFlags >= 0 && fcntl(listener, F_SETFL, listenerFlags | O_NONBLOCK) == 0;
    setsockopt(listener, SOL_SOCKET, SO_REUSEADDR, &enabled, sizeof(enabled));
    struct sockaddr_in address; memset(&address, 0, sizeof(address));
    address.sin_len = sizeof(address); address.sin_family = AF_INET; address.sin_port = htons(52742); address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    [_socketLock lock];
    BOOL listening = nonblocking && self.generation == generation && bind(listener, (struct sockaddr *)&address, sizeof(address)) == 0 && listen(listener, 1) == 0;
    [_socketLock unlock];
    if (!listening) {
        result = @"No se pudo abrir el puerto USB 52742.";
    } else {
        [self status:@"Enlace listo. Conecta el host Relevo Native por USB." generation:generation];
        while (self.generation == generation) {
            struct pollfd wait = {listener, POLLIN, 0};
            int available = poll(&wait, 1, 250);
            if (available < 0 && errno == EINTR) continue;
            if (available < 0) break;
            if (!available) continue;
            if (self.generation != generation) break;
            if (!(wait.revents & POLLIN)) break;
            peer = accept(listener, NULL, NULL);
            if (peer >= 0) break;
            if (errno != EINTR && errno != EAGAIN && errno != EWOULDBLOCK) break;
        }
        if (peer >= 0) {
            [_socketLock lock];
            if (self.generation == generation) _peer = peer;
            else shutdown(peer, SHUT_RDWR);
            // Exactly one peer: no listening socket remains while connected.
            if (_listener == listener) _listener = -1;
            close(listener); listener = -1;
            [_socketLock unlock];
            int peerFlags = fcntl(peer, F_GETFL, 0);
            if (peerFlags >= 0) fcntl(peer, F_SETFL, peerFlags & ~O_NONBLOCK);
            setsockopt(peer, SOL_SOCKET, SO_NOSIGPIPE, &enabled, sizeof(enabled));
            struct timeval timeout = {3, 0}; setsockopt(peer, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof(timeout));
            struct timeval readTimeout = {15, 0}; setsockopt(peer, SOL_SOCKET, SO_RCVTIMEO, &readTimeout, sizeof(readTimeout));
            if (self.generation == generation && [self sendJSON:@{@"protocol":@1, @"width":@1024, @"height":@768} type:1 fd:peer generation:generation]) {
                result = [self receive:peer generation:generation];
            }
        }
    }
    [_writeLock lock];
    [_socketLock lock];
    if (_listener == listener) _listener = -1;
    if (_peer == peer) _peer = -1;
    if (listener >= 0) close(listener);
    if (peer >= 0) close(peer);
    // Invalidate queued writes before this descriptor can be reused by a later session.
    BOOL wasCurrent=self.generation==generation;
    if(wasCurrent) self.generation=generation+1;
    NSUInteger endedGeneration=self.generation;
    [_socketLock unlock];
    [_writeLock unlock];
    if(!wasCurrent)return;
    dispatch_async(dispatch_get_main_queue(), ^{ if(self.generation==endedGeneration){self.touchAccepted=NO;self.cameraAccepted=NO;self.audioAccepted=NO;self.multitouchAccepted=NO;self.keyboardAccepted=NO;self.microphoneAccepted=NO; self.pointerAccepted=NO; self.inputFocusAccepted=NO; self.inputModesAccepted=NO; [self invalidateInputFocus];if(self.extendedCapabilitiesChanged)self.extendedCapabilitiesChanged(NO,NO,NO);if(self.capabilitiesChanged)self.capabilitiesChanged(NO,NO,NO);} });
    [self status:result generation:endedGeneration];
}
- (NSString *)receive:(int)fd generation:(NSUInteger)generation {
    BOOL ready = NO;
    while (self.generation == generation) {
        @autoreleasepool {
            uint8_t header[16];
            if (!RVReadAll(fd, header, sizeof(header))) break;
            uint16_t type = RVRead16(header + 4), flags = RVRead16(header + 6);
            uint32_t sequence = RVRead32(header + 8), length = RVRead32(header + 12);
            if (memcmp(header, "RVL1", 4) || flags) return @"Protocolo inválido: cabecera USB.";
            BOOL allowed = (type == 2 && !ready && length > 0 && length <= 4096) || (type == 3 && ready && length > 0 && length <= 4 * 1024 * 1024) || ((type == 6 || type == 7) && length == 0) || (type==14 && ready && self.inputFocusAccepted && length>0 && length<=4096) || (type==9 && ready && self.audioAccepted && length>=10 && length<=890);
            if (!allowed) return @"Protocolo inválido: tipo, tamaño u orden no admitido.";
            NSMutableData *payload = [NSMutableData dataWithLength:length];
            if (length && !RVReadAll(fd, payload.mutableBytes, length)) break;
            if (type == 6) { if (![self send:7 sequence:sequence data:nil fd:fd generation:generation]) break; }
            else if (type == 2) {
                id json = [NSJSONSerialization JSONObjectWithData:payload options:0 error:nil];
                if (![json isKindOfClass:[NSDictionary class]]) return @"Protocolo inválido: configuración.";
                id width = json[@"width"], height = json[@"height"], fps = json[@"fps"];
                if (![width isKindOfClass:[NSNumber class]] || ![height isKindOfClass:[NSNumber class]] || ![fps isKindOfClass:[NSNumber class]] || CFGetTypeID((__bridge CFTypeRef)width) == CFBooleanGetTypeID() || CFGetTypeID((__bridge CFTypeRef)height) == CFBooleanGetTypeID() || CFGetTypeID((__bridge CFTypeRef)fps) == CFBooleanGetTypeID() || [width doubleValue] != 1024 || [height doubleValue] != 768 || [fps doubleValue] < 1 || [fps doubleValue] > 60 || [fps doubleValue] != [fps integerValue]) return @"Configuración no admitida: usa 1024 × 768 y 1–60 FPS.";
                for (NSString *key in @[@"touch",@"camera",@"audio",@"multitouch",@"keyboard",@"microphone",@"inputFocus",@"pointer",@"inputModes"]) if (json[key] && CFGetTypeID((__bridge CFTypeRef)json[key])!=CFBooleanGetTypeID()) return @"Capacidad inválida.";
                self.inputModesAccepted=[json[@"inputModes"] boolValue];
                self.inputFocusAccepted=[json[@"inputFocus"] boolValue] && [json[@"keyboard"] boolValue]; self.pointerAccepted=[json[@"pointer"] boolValue];
                if([json[@"inputFocus"] boolValue] && !self.inputFocusAccepted)return @"El foco requiere teclado negociado.";
                self.multitouchAccepted=[json[@"multitouch"] boolValue]; self.keyboardAccepted=[json[@"keyboard"] boolValue]; self.microphoneAccepted=[json[@"microphone"] boolValue];
                self.touchAccepted=[json[@"touch"] boolValue]; self.cameraAccepted=[json[@"camera"] boolValue];self.audioAccepted=[json[@"audio"] boolValue];
                dispatch_async(dispatch_get_main_queue(), ^{ if(self.generation==generation && self.capabilitiesChanged)self.capabilitiesChanged(self.touchAccepted,self.cameraAccepted,self.audioAccepted); });
                NSMutableDictionary *status=[@{@"state":@"ready",@"touch":@(self.touchAccepted),@"camera":@(self.cameraAccepted),@"audio":@(self.audioAccepted)} mutableCopy];
                // Older strict v0.4 hosts must not receive extension fields they did not request.
                for(NSString *key in @[@"multitouch",@"keyboard",@"microphone",@"inputFocus",@"pointer",@"inputModes"])if(json[key])status[key]=json[key];
                if (![self sendJSON:status type:5 fd:fd generation:generation]) break;
                ready = YES;
                dispatch_async(dispatch_get_main_queue(), ^{if(self.generation==generation && self.extendedCapabilitiesChanged)self.extendedCapabilitiesChanged(self.multitouchAccepted,self.keyboardAccepted,self.microphoneAccepted);});
                [self status:@"Enlace USB conectado · pantalla 1024 × 768." generation:generation];
            } else if(type==14) {
                id json=[NSJSONSerialization JSONObjectWithData:payload options:0 error:nil];
                if(![json isKindOfClass:[NSDictionary class]] || [json count]!=2)return @"Foco inválido.";
                id editable=json[@"editable"], request=json[@"sequence"];
                if(!editable || CFGetTypeID((__bridge CFTypeRef)editable)!=CFBooleanGetTypeID() || ![request isKindOfClass:[NSNumber class]] || CFGetTypeID((__bridge CFTypeRef)request)==CFBooleanGetTypeID() || [request doubleValue]<1 || [request doubleValue]>UINT32_MAX || [request doubleValue]!=[request unsignedIntValue] || sequence!=[request unsignedIntValue])return @"Foco inválido.";
                uint32_t requestID=[request unsignedIntValue];BOOL enabled=[editable boolValue];
                dispatch_async(dispatch_get_main_queue(), ^{if(self.generation==generation && self.latestTouchSequence==requestID && self.focusChanged)self.focusChanged(enabled,requestID);});
            } else if(type==9) {
                // Audio callback must be thread-safe and consumes synchronously without dispatch FIFO.
                const uint8_t *p=payload.bytes;
                if(length<10 || length>890 || ((length-8)&1) || p[0]!=0x22 || p[1]!=0x56 || p[2] || p[3] || p[4]!=1 || p[5] || p[6]!=16 || p[7]) return @"Formato de audio USB inválido.";
                if(self.audioReceived) self.audioReceived(payload);
            } else if (type == 3) {
                // Decode on reader; only one image is dispatched, with no image FIFO.
                CGImageSourceRef source = CGImageSourceCreateWithData((__bridge CFDataRef)payload, NULL);
                NSDictionary *properties = source ? CFBridgingRelease(CGImageSourceCopyPropertiesAtIndex(source, 0, NULL)) : nil;
                BOOL jpeg = source && CGImageSourceGetType(source) && CFEqual(CGImageSourceGetType(source), CFSTR("public.jpeg"));
                BOOL dimensions = [properties[(__bridge NSString *)kCGImagePropertyPixelWidth] integerValue] == 1024 && [properties[(__bridge NSString *)kCGImagePropertyPixelHeight] integerValue] == 768;
                if (!jpeg || !dimensions) { if (source) CFRelease(source); return @"Imagen inválida: se esperaba JPEG de 1024 × 768."; }
                CGImageRef decoded = CGImageSourceCreateImageAtIndex(source, 0, (__bridge CFDictionaryRef)@{(__bridge NSString *)kCGImageSourceShouldCacheImmediately:@YES});
                CFRelease(source);
                if (!decoded) return @"Imagen inválida: se esperaba JPEG de 1024 × 768.";
                UIImage *image = [UIImage imageWithCGImage:decoded];
                CGImageRelease(decoded);
                if (!image || CGImageGetWidth(image.CGImage) != 1024 || CGImageGetHeight(image.CGImage) != 768) return @"Imagen inválida: se esperaba JPEG de 1024 × 768.";
                __block BOOL applied = NO;
                dispatch_sync(dispatch_get_main_queue(), ^{
                    if (self.generation == generation && self.imageReceived) { self.imageReceived(image); applied = YES; }
                });
                // This acknowledges image assignment, not physical presentation.
                if (!applied || ![self send:10 sequence:sequence data:nil fd:fd generation:generation]) break;
            }
        }
    }
    return @"Enlace USB desconectado. Pulsa Iniciar para volver a conectar.";
}
- (void)dealloc {
    if (_listener >= 0) { shutdown(_listener, SHUT_RDWR); close(_listener); }
    if (_peer >= 0) { shutdown(_peer, SHUT_RDWR); close(_peer); }
}
@end
