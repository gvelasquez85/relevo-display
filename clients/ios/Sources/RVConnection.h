#import <Foundation/Foundation.h>
#import <UIKit/UIKit.h>
@interface RVConnection : NSObject
// Call public methods on main. Callbacks arrive on main; stop invalidates pending callbacks.
@property(nonatomic, copy) void (^statusChanged)(NSString *status);
@property(nonatomic, copy) void (^imageReceived)(UIImage *image);
@property(nonatomic, copy) void (^audioReceived)(NSData *packet);
@property(nonatomic, copy) void (^capabilitiesChanged)(BOOL touch, BOOL camera, BOOL audio);
@property(nonatomic, copy) void (^extendedCapabilitiesChanged)(BOOL multitouch, BOOL keyboard, BOOL microphone);
@property(atomic, readonly) BOOL multitouchAccepted, keyboardAccepted, microphoneAccepted;
@property(nonatomic, copy) void (^focusChanged)(BOOL editable, uint32_t sequence);
@property(atomic, readonly) BOOL pointerAccepted, inputFocusAccepted, inputModesAccepted;
@property(atomic, readonly) uint32_t latestTouchSequence;
- (void)invalidateInputFocus;
- (void)sendInputMode:(NSString *)mode;
- (void)sendPointer:(NSDictionary *)pointer;
- (void)sendContacts:(NSArray *)contacts;
- (void)sendKey:(NSDictionary *)key;
- (void)sendMicrophone:(NSData *)packet;
@property(atomic, readonly) BOOL touchAccepted;
@property(atomic, readonly) BOOL cameraAccepted;
@property(atomic, readonly) BOOL audioAccepted;
- (void)sendTouch:(NSString *)phase x:(double)x y:(double)y;
- (void)sendCamera:(NSData *)jpeg;
- (void)start;
// Outbound TLS 1.2 with a one-session certificate pin and pairing token.
- (BOOL)startWiFiWithPairingURI:(NSString *)uri;
- (void)stop;
@end
