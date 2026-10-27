// Regression helper: decode the actual rendered GUI pixels with Apple's Vision.
#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <ImageIO/ImageIO.h>
int main(void){@autoreleasepool {
 NSData *input=[[NSFileHandle fileHandleWithStandardInput]readDataToEndOfFile];
 NSDictionary *options=[NSJSONSerialization JSONObjectWithData:input options:0 error:nil];
 if(![options isKindOfClass:[NSDictionary class]])return 2;
 NSData *png=[[NSData alloc]initWithBase64EncodedString:options[@"png"] options:0];
 CGImageSourceRef source=CGImageSourceCreateWithData((__bridge CFDataRef)png,NULL);
 CGImageRef image=source?CGImageSourceCreateImageAtIndex(source,0,NULL):NULL;
 if(source)CFRelease(source);if(!image)return 3;
 VNDetectBarcodesRequest *request=[VNDetectBarcodesRequest new];request.symbologies=@[VNBarcodeSymbologyQR];
 VNImageRequestHandler *handler=[[VNImageRequestHandler alloc]initWithCGImage:image options:@{}];
 NSError *error=nil;BOOL ok=[handler performRequests:@[request] error:&error];CGImageRelease(image);
 if(!ok || error)return 4;
 if(request.results.count!=1)return 5;
 VNBarcodeObservation *found=request.results.firstObject;
 if(![found.payloadStringValue isEqualToString:options[@"expected"]])return 6;
 puts("DECODED");return 0;
}}
