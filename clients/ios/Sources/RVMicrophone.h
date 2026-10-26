#import <Foundation/Foundation.h>
@interface RVMicrophone : NSObject
// Set callbacks before start; packet callback executes on the AudioQueue thread.
@property(nonatomic,copy) void (^packetReady)(NSData *packet);
@property(nonatomic,copy) void (^failed)(void);
- (BOOL)start;
- (void)stop;
@end
