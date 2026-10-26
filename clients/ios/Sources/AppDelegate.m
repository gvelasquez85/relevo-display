#import "AppDelegate.h"
#import "DisplayViewController.h"
@implementation AppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)options {
    self.window = [[UIWindow alloc] initWithFrame:[UIScreen mainScreen].bounds];
    self.window.rootViewController = [DisplayViewController new];
    [self.window makeKeyAndVisible];
    return YES;
}
@end
