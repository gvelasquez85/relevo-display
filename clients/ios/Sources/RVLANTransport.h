#import <Foundation/Foundation.h>
@interface RVLANTransport : NSObject
// Exact in-memory certificate pin; no certificate or pairing token is persisted.
- (instancetype)initWithSocket:(int)socket pin:(NSData *)pin;
- (BOOL)authenticateToken:(NSData *)token;
- (BOOL)read:(void *)buffer length:(size_t)length;
- (BOOL)write:(const void *)buffer length:(size_t)length;
@end
