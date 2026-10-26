#import <Foundation/Foundation.h>
@interface RVAudioPlayer : NSObject
- (BOOL)start;
- (void)stop;
- (BOOL)consume:(NSData *)packet;
@end
