// macOS interoperability harness: credentials arrive only through stdin.
#import <Foundation/Foundation.h>
#import "RVLANTransport.h"
#import <arpa/inet.h>
#import <sys/socket.h>
#import <unistd.h>
#import <string.h>
static NSData *decode(NSString *text){NSMutableData *out=[NSMutableData new];for(NSUInteger i=0;i+1<text.length;i+=2){unsigned value=0;NSScanner *scanner=[NSScanner scannerWithString:[text substringWithRange:NSMakeRange(i,2)]];if(![scanner scanHexInt:&value])return nil;uint8_t b=value;[out appendBytes:&b length:1];}return out;}
int main(void){@autoreleasepool {
 NSData *input=[[NSFileHandle fileHandleWithStandardInput]readDataToEndOfFile];
 NSDictionary *options=[NSJSONSerialization JSONObjectWithData:input options:0 error:nil];
 if(![options isKindOfClass:[NSDictionary class]])return 2;
 int fd=socket(AF_INET,SOCK_STREAM,0);if(fd<0)return 3;
 int yes=1;setsockopt(fd,SOL_SOCKET,SO_NOSIGPIPE,&yes,sizeof(yes));
 struct sockaddr_in address={0};address.sin_len=sizeof(address);address.sin_family=AF_INET;address.sin_port=htons([options[@"port"]intValue]);
 if(inet_pton(AF_INET,"127.0.0.1",&address.sin_addr)!=1 || connect(fd,(struct sockaddr *)&address,sizeof(address))!=0){close(fd);return 4;}
 RVLANTransport *transport=[[RVLANTransport alloc]initWithSocket:fd pin:decode(options[@"pin"])];
 BOOL authenticated=transport && [transport authenticateToken:decode(options[@"token"])];
 if([options[@"reject"]boolValue]){close(fd);if(authenticated)return 5;puts("REJECTED");return 0;}
 if(!authenticated){close(fd);return 6;}
 uint8_t prefix[4];if(![transport read:prefix length:4] || memcmp(prefix,"RVL1",4)){close(fd);return 7;}
 if(![transport write:"RVL1" length:4]){close(fd);return 8;}
 NSMutableData *payload=[NSMutableData dataWithLength:512*1024];uint8_t *bytes=payload.mutableBytes;
 for(NSUInteger i=0;i<payload.length;i++)bytes[i]=(uint8_t)(i%251);
 int size=4096;setsockopt(fd,SOL_SOCKET,SO_SNDBUF,&size,sizeof(size));
 if(![transport write:bytes length:payload.length]){close(fd);return 9;}
 memset(bytes,0,payload.length);if(![transport read:bytes length:payload.length]){close(fd);return 10;}
 for(NSUInteger i=0;i<payload.length;i++)if(bytes[i]!=(uint8_t)(i%251)){close(fd);return 11;}
 close(fd);puts("OK");return 0;
}}
