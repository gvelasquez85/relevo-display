#import "DisplayViewController.h"
#import <AVFoundation/AVFoundation.h>
#import "RVConnection.h"
#import "RVAudioPlayer.h"
#import "RVMicrophone.h"
#import <CoreImage/CoreImage.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <math.h>
#import <stdint.h>
#import <string.h>

@interface RVKeyboardField : UITextField
@property(nonatomic,copy) void (^emptyDelete)(void);
@end
@implementation RVKeyboardField
- (void)deleteBackward {BOOL empty=!self.text.length && !self.markedTextRange;[super deleteBackward];if(empty && self.emptyDelete)self.emptyDelete();}
@end

@interface DisplayViewController () <AVAudioPlayerDelegate, AVCaptureVideoDataOutputSampleBufferDelegate, UITextFieldDelegate>
@property(atomic) NSUInteger cameraGeneration;
@property(atomic) BOOL cameraUSBEnabled;
@end

@implementation DisplayViewController {
    RVMicrophone *_microphone;
    NSUInteger _microphoneRequest;
    UIButton *_micButton;
    UISegmentedControl *_inputMode;
    UIView *_trackpad;
    UILabel *_modeHint;
    BOOL _remoteEditable, _pointerDragging;
    UILabel *_micStatus;
    UITextField *_keyboard;
    NSMapTable *_contacts;
    NSMutableDictionary *_contactPoints;
    NSMutableSet *_heldKeys;
    UIView *_keyboardAccessory;
    UIView *_preview;
    UILabel *_title, *_cameraStatus, *_audioStatus, *_note;
    UIButton *_cameraStart, *_cameraStop, *_audioStart, *_audioStop;
    AVCaptureSession *_session;
    AVCaptureVideoPreviewLayer *_previewLayer;
    dispatch_queue_t _captureQueue;
    AVAudioPlayer *_player;
    BOOL _visible;
    RVAudioPlayer *_usbAudio;
    UIButton *_usbAudioStart, *_usbAudioStop, *_usbCameraToggle, *_cameraSwitch;
    AVCaptureVideoDataOutput *_videoOutput;
    CIContext *_imageContext;
    CFTimeInterval _lastCameraFrame;
    UITouch *_remoteTouch;
    CGPoint _lastTouch;
    BOOL _cameraUSBRequested;
    RVConnection *_link;
    UIImageView *_display;
    UILabel *_linkStatus;
    UIButton *_linkStart, *_linkStop;
}
- (UILabel *)label:(NSString *)text size:(CGFloat)size {
    UILabel *label = [UILabel new];
    label.text = text; label.textColor = [UIColor whiteColor];
    label.font = [UIFont systemFontOfSize:size]; label.numberOfLines = 0;
    [self.view addSubview:label]; return label;
}
- (UIButton *)button:(NSString *)text action:(SEL)action {
    UIButton *button = [UIButton buttonWithType:UIButtonTypeSystem];
    [button setTitle:text forState:UIControlStateNormal];
    [button setTitleColor:[UIColor whiteColor] forState:UIControlStateNormal];
    button.backgroundColor = [UIColor colorWithRed:0.12 green:0.30 blue:0.48 alpha:1];
    button.titleLabel.font = [UIFont boldSystemFontOfSize:20]; button.titleLabel.adjustsFontSizeToFitWidth=YES; button.titleLabel.minimumScaleFactor=0.65;
    button.layer.cornerRadius = 8;
    [button addTarget:self action:action forControlEvents:UIControlEventTouchUpInside];
    [self.view addSubview:button]; return button;
}
- (void)viewDidLoad {
    [super viewDidLoad];
    self.view.backgroundColor = [UIColor colorWithWhite:0.07 alpha:1];
    _display = [UIImageView new]; _display.contentMode = UIViewContentModeScaleAspectFit;
    _display.backgroundColor = [UIColor blackColor]; _display.accessibilityLabel = @"Pantalla recibida por USB";
    [self.view addSubview:_display];
    _display.userInteractionEnabled=YES; _display.multipleTouchEnabled=YES;
    _contacts=[NSMapTable strongToStrongObjectsMapTable]; _contactPoints=[NSMutableDictionary new]; _heldKeys=[NSMutableSet new];
    _usbAudio=[RVAudioPlayer new];
    _link = [RVConnection new];
    __weak DisplayViewController *weakSelf = self;
    _link.statusChanged = ^(NSString *status) { DisplayViewController *controller = weakSelf; if (controller) controller->_linkStatus.text = status; };
    _link.imageReceived = ^(UIImage *image) { DisplayViewController *controller = weakSelf; if (controller) { [UIApplication sharedApplication].idleTimerDisabled=YES; controller->_display.image = image; controller->_display.hidden = controller->_inputMode.selectedSegmentIndex!=0; [controller.view setNeedsLayout]; } };
    RVAudioPlayer *usbAudio=_usbAudio;
    _link.audioReceived=^(NSData *packet){
        if (![usbAudio consume:packet]) dispatch_async(dispatch_get_main_queue(), ^{
            DisplayViewController *controller=weakSelf;
            if (controller && controller->_link.audioAccepted) {
                [controller stopUSBAudio];
                controller->_audioStatus.text=@"Falló la reproducción USB. Pulsa Escuchar Windows para reintentar.";
            }
        });
    };
    _link.capabilitiesChanged=^(BOOL touch,BOOL camera,BOOL audio){
        DisplayViewController *controller=weakSelf; if(!controller)return;
        if(!touch && !camera && !audio) [UIApplication sharedApplication].idleTimerDisabled=NO;
        controller.cameraUSBEnabled=camera && controller->_cameraUSBRequested;
        if(!touch) controller->_remoteTouch=nil;
        if(!audio) [controller->_usbAudio stop];
        if(!camera) {controller->_cameraUSBRequested=NO; [controller->_usbCameraToggle setTitle:@"Enviar cámara USB" forState:UIControlStateNormal];}
        controller->_usbAudioStart.enabled=audio;
        controller->_usbCameraToggle.enabled=camera;
    };
    _microphone=[RVMicrophone new];
    RVConnection *connection=_link;
    _microphone.packetReady=^(NSData *packet){[connection sendMicrophone:packet];};
    _microphone.failed=^{DisplayViewController *controller=weakSelf;if(controller){[controller stopMicrophone];controller->_micStatus.text=@"Falló el micrófono. Pulsa de nuevo para reintentar.";}};
    _link.extendedCapabilitiesChanged=^(BOOL multitouch,BOOL keyboard,BOOL microphone){
        DisplayViewController *controller=weakSelf;if(!controller)return;
        [controller cancelRemoteTouch];
        if(!keyboard)controller->_remoteEditable=NO;
        if(!controller->_link.pointerAccepted)[controller releasePointer];
        [controller updateInputMode];
        controller->_micButton.enabled=microphone;
        if(!keyboard){[controller releaseKeyboard];[controller->_keyboard resignFirstResponder];}
        if(!microphone)[controller stopMicrophone];
    };
    _link.focusChanged=^(BOOL editable,uint32_t sequence){
        DisplayViewController *controller=weakSelf;
        if(!controller || controller->_inputMode.selectedSegmentIndex!=0 || controller->_link.latestTouchSequence!=sequence)return;
        controller->_remoteEditable=editable;
        if(editable && controller->_visible)[controller->_keyboard becomeFirstResponder];
        else {[controller releaseKeyboard];[controller->_keyboard resignFirstResponder];}
    };
    _captureQueue = dispatch_queue_create("com.relevodisplay.camera", DISPATCH_QUEUE_SERIAL);
    _session = [AVCaptureSession new];
    _preview = [UIView new]; _preview.backgroundColor = [UIColor blackColor];
    _preview.accessibilityLabel = @"Vista previa local de la cámara";
    [self.view addSubview:_preview];
    _previewLayer = [AVCaptureVideoPreviewLayer layerWithSession:_session];
    _previewLayer.videoGravity = AVLayerVideoGravityResizeAspect;
    [_preview.layer addSublayer:_previewLayer];
    _display.hidden = YES;
    [self.view bringSubviewToFront:_display];
    _title = [self label:@"Relevo Display · cámara y sonido" size:26];
    _cameraStatus = [self label:@"Cámara detenida. Pulsa Iniciar cámara para autorizarla." size:18];
    _audioStatus = [self label:@"Audio detenido. El tono dura 5 segundos; empieza con volumen bajo." size:18];
    _note = [self label:@"Pantalla: teclado sólo al tocar un campo editable. Teclado y Mouse: controles dedicados sin imagen. Audio y micrófono se prueban por separado." size:16];
    _linkStatus = [self label:@"USB detenido." size:16];
    _linkStart = [self button:@"Iniciar enlace USB" action:@selector(startLink)];
    _linkStop = [self button:@"Detener enlace USB" action:@selector(stopLink)];
    _cameraStart = [self button:@"Iniciar cámara" action:@selector(startCamera)];
    _cameraStop = [self button:@"Detener cámara" action:@selector(stopCamera)];
    _audioStart = [self button:@"Reproducir tono" action:@selector(playTone)];
    _audioStop = [self button:@"Detener sonido" action:@selector(stopTone)];
    _cameraSwitch=[self button:@"Cambiar cámara" action:@selector(switchCamera)];
    _usbAudioStart=[self button:@"Escuchar Windows" action:@selector(startUSBAudio)];
    _usbAudioStop=[self button:@"Parar audio USB" action:@selector(stopUSBAudio)];
    _usbCameraToggle=[self button:@"Enviar cámara USB" action:@selector(toggleUSBCamera)];
    _inputMode=[[UISegmentedControl alloc]initWithItems:@[@"Pantalla",@"Teclado",@"Mouse"]];
    _inputMode.selectedSegmentIndex=0;_inputMode.tintColor=[UIColor whiteColor];
    [_inputMode addTarget:self action:@selector(inputModeChanged:) forControlEvents:UIControlEventValueChanged];
    [self.view addSubview:_inputMode];
    _trackpad=[UIView new];_trackpad.backgroundColor=[UIColor colorWithWhite:0.16 alpha:1];
    _trackpad.accessibilityLabel=@"Panel táctil del mouse: arrastrar para mover, toque para clic, dos dedos para desplazar";
    _trackpad.hidden=YES;[self.view addSubview:_trackpad];
    UIPanGestureRecognizer *move=[[UIPanGestureRecognizer alloc]initWithTarget:self action:@selector(pointerMove:)];move.maximumNumberOfTouches=1;[_trackpad addGestureRecognizer:move];
    UIPanGestureRecognizer *scroll=[[UIPanGestureRecognizer alloc]initWithTarget:self action:@selector(pointerScroll:)];scroll.minimumNumberOfTouches=2;scroll.maximumNumberOfTouches=2;[_trackpad addGestureRecognizer:scroll];
    UITapGestureRecognizer *tap=[[UITapGestureRecognizer alloc]initWithTarget:self action:@selector(pointerTap:)];[_trackpad addGestureRecognizer:tap];
    UITapGestureRecognizer *right=[[UITapGestureRecognizer alloc]initWithTarget:self action:@selector(pointerRightTap:)];right.numberOfTouchesRequired=2;[_trackpad addGestureRecognizer:right];
    [tap requireGestureRecognizerToFail:right];
    UILongPressGestureRecognizer *drag=[[UILongPressGestureRecognizer alloc]initWithTarget:self action:@selector(pointerDrag:)];[_trackpad addGestureRecognizer:drag];
    [move requireGestureRecognizerToFail:drag];
    _modeHint=[self label:@"" size:22];_modeHint.textAlignment=NSTextAlignmentCenter;_modeHint.userInteractionEnabled=NO;
    _micButton=[self button:@"Enviar micrófono" action:@selector(requestMicrophone)];
    _micStatus=[self label:@"Micrófono apagado." size:16];
    _micButton.enabled=NO;
    RVKeyboardField *keyboard=[RVKeyboardField new];
    keyboard.emptyDelete=^{DisplayViewController *controller=weakSelf;if(controller)[controller->_link sendKey:@{@"kind":@"press",@"code":@"Backspace"}];};
    _keyboard=keyboard;_keyboard.delegate=self;
    _keyboard.autocorrectionType=UITextAutocorrectionTypeNo;_keyboard.spellCheckingType=UITextSpellCheckingTypeNo;
    _keyboard.autocapitalizationType=UITextAutocapitalizationTypeNone;
    _keyboard.accessibilityLabel=@"Escribir en Windows";
    [_keyboard addTarget:self action:@selector(keyboardChanged:) forControlEvents:UIControlEventEditingChanged];
    [self.view addSubview:_keyboard];
    UIScrollView *keys=[[UIScrollView alloc]initWithFrame:CGRectMake(0,0,1024,44)];keys.backgroundColor=[UIColor darkGrayColor];
    NSArray *codes=@[@"Control",@"Alt",@"Shift",@"Meta",@"Escape",@"Tab",@"ArrowLeft",@"ArrowRight",@"ArrowUp",@"ArrowDown",@"Delete",@"Home",@"End",@"PageUp",@"PageDown",@"KeyC",@"KeyV",@"KeyA",@"Cerrar"];
    for(NSUInteger i=0;i<codes.count;i++){UIButton *b=[UIButton buttonWithType:UIButtonTypeSystem];[b setTitle:codes[i] forState:UIControlStateNormal];[b setTitleColor:[UIColor whiteColor] forState:UIControlStateNormal];b.frame=CGRectMake(i*85,0,82,44);[b addTarget:self action:@selector(controlKey:) forControlEvents:UIControlEventTouchUpInside];[keys addSubview:b];}
    keys.contentSize=CGSizeMake(codes.count*85,44);_keyboard.inputAccessoryView=keys;_keyboardAccessory=keys;
    _usbAudioStart.enabled=NO; _usbCameraToggle.enabled=NO;
    [self updateInputMode];
    NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
    [nc addObserver:self selector:@selector(background:) name:UIApplicationDidEnterBackgroundNotification object:nil];
    [nc addObserver:self selector:@selector(cameraInterrupted:) name:AVCaptureSessionWasInterruptedNotification object:_session];
    [nc addObserver:self selector:@selector(cameraFailed:) name:AVCaptureSessionRuntimeErrorNotification object:_session];
    [nc addObserver:self selector:@selector(audioInterrupted:) name:AVAudioSessionInterruptionNotification object:nil];
}
- (void)viewDidAppear:(BOOL)animated { [super viewDidAppear:animated]; _visible = YES; }
- (void)viewWillDisappear:(BOOL)animated {
    [super viewWillDisappear:animated]; _visible = NO; [self stopCamera]; [self stopTone]; [self stopLink];
}
- (void)viewDidLayoutSubviews {
    [super viewDidLayoutSubviews];
    CGFloat w = self.view.bounds.size.width, h = self.view.bounds.size.height;
    CGFloat gap = 16, side = MIN(340, w * 0.40), left = w - side - gap;
    _title.frame = CGRectMake(gap, 16, w - 2 * gap, 48);
    _preview.frame = CGRectMake(gap, 80, MAX(0, left - 2 * gap), MAX(0, h - 104));
    _previewLayer.frame = _preview.bounds;
    _display.frame = _preview.frame;
    if(_display.image && _session.running) {
        _preview.frame=CGRectMake(gap+8,88,160,120);
        _previewLayer.frame=_preview.bounds;
        [self.view bringSubviewToFront:_preview];
    } else if(_display.image) [self.view bringSubviewToFront:_display];
    AVCaptureConnection *connection = _previewLayer.connection;
    if (connection.isVideoOrientationSupported) {
        connection.videoOrientation = [UIApplication sharedApplication].statusBarOrientation == UIInterfaceOrientationLandscapeLeft ? AVCaptureVideoOrientationLandscapeLeft : AVCaptureVideoOrientationLandscapeRight;
    }
    _cameraStatus.frame=CGRectMake(left,76,side,58);
    _cameraStart.frame=CGRectMake(left,138,(side-8)/2,38);
    _cameraStop.frame=CGRectMake(left+(side+8)/2,138,(side-8)/2,38);
    _cameraSwitch.frame=CGRectMake(left,182,side,38);
    _usbCameraToggle.frame=CGRectMake(left,226,side,38);
    _audioStatus.frame=CGRectMake(left,270,side,58);
    _audioStart.frame=CGRectMake(left,334,side,38);
    _audioStop.frame=CGRectMake(left,378,side,38);
    _usbAudioStart.frame=CGRectMake(left,422,side,38);
    _usbAudioStop.frame=CGRectMake(left,466,side,38);
    _linkStart.frame=CGRectMake(left,510,side,38);
    _linkStop.frame=CGRectMake(left,554,side,38);
    _linkStatus.frame=CGRectMake(left,598,side,58);
    _inputMode.frame=CGRectMake(gap,h-52,280,36);
    _trackpad.frame=_display.frame;_modeHint.frame=CGRectInset(_display.frame,20,30);
    _display.hidden=_inputMode.selectedSegmentIndex!=0 || !_display.image;
    _trackpad.hidden=_inputMode.selectedSegmentIndex!=2;
    _modeHint.hidden=_inputMode.selectedSegmentIndex==0;
    if(_inputMode.selectedSegmentIndex!=0){_preview.hidden=YES;[self.view bringSubviewToFront:_trackpad];[self.view bringSubviewToFront:_modeHint];}
    else _preview.hidden=NO;
    _micButton.frame=CGRectMake(gap+288,h-52,180,36);
    _micStatus.frame=CGRectMake(gap,h-90,MAX(0,left-2*gap),34);
    _keyboard.frame=CGRectMake(0,0,1,1);
    _note.frame=CGRectMake(left,660,side,MAX(0,h-666));
}
- (void)startLink {
    if (!_visible || [UIApplication sharedApplication].applicationState != UIApplicationStateActive) return;
    _display.hidden = NO; [_link start];
}
- (void)stopLink { [UIApplication sharedApplication].idleTimerDisabled=NO; [self cancelRemoteTouch]; [self releasePointer]; _remoteEditable=NO; [_link invalidateInputFocus]; [self releaseKeyboard]; [_keyboard resignFirstResponder]; [self stopMicrophone]; [self stopUSBAudio]; self.cameraUSBEnabled=NO; _cameraUSBRequested=NO; [_usbCameraToggle setTitle:@"Enviar cámara USB" forState:UIControlStateNormal]; [_link stop]; _display.image = nil; _display.hidden = YES; _linkStatus.text = @"USB detenido."; }
- (void)startCamera {
    if (!_visible || [UIApplication sharedApplication].applicationState != UIApplicationStateActive) return;
    NSUInteger generation = ++self.cameraGeneration;
    _cameraStatus.text = @"Preparando cámara…";
    AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
    if (status == AVAuthorizationStatusNotDetermined) {
        __weak DisplayViewController *weakSelf = self;
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeVideo completionHandler:^(BOOL granted) {
            dispatch_async(dispatch_get_main_queue(), ^{
                DisplayViewController *controller = weakSelf;
                if (!controller || controller.cameraGeneration != generation) return;
                if (granted) [controller configureCamera:generation];
                else controller->_cameraStatus.text = @"Cámara no autorizada. Puedes habilitarla en Ajustes > Privacidad > Cámara.";
            });
        }];
    } else if (status == AVAuthorizationStatusAuthorized) {
        [self configureCamera:generation];
    } else {
        _cameraStatus.text = @"Cámara bloqueada o restringida. Revisa Ajustes > Privacidad > Cámara.";
    }
}
- (void)configureCamera:(NSUInteger)generation {
    if (!_visible || [UIApplication sharedApplication].applicationState != UIApplicationStateActive) {
        [self stopCamera]; return;
    }
    dispatch_async(_captureQueue, ^{
        if (self.cameraGeneration != generation) return;
        NSError *error = nil;
        if (self->_session.inputs.count == 0) {
            AVCaptureDevice *device = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
            AVCaptureDeviceInput *input = device ? [AVCaptureDeviceInput deviceInputWithDevice:device error:&error] : nil;
            if (!input || ![self->_session canAddInput:input]) {
                NSString *message = error.localizedDescription ?: @"Este dispositivo no tiene una cámara disponible.";
                dispatch_async(dispatch_get_main_queue(), ^{
                    if (self.cameraGeneration == generation) self->_cameraStatus.text = message;
                }); return;
            }
            [self->_session beginConfiguration];
            if ([self->_session canSetSessionPreset:AVCaptureSessionPreset640x480]) self->_session.sessionPreset = AVCaptureSessionPreset640x480;
            [self->_session addInput:input];
            self->_videoOutput=[AVCaptureVideoDataOutput new];
            self->_videoOutput.alwaysDiscardsLateVideoFrames=YES;
            self->_videoOutput.videoSettings=@{(id)kCVPixelBufferPixelFormatTypeKey:@(kCVPixelFormatType_32BGRA)};
            [self->_videoOutput setSampleBufferDelegate:self queue:self->_captureQueue];
            if([self->_session canAddOutput:self->_videoOutput]) [self->_session addOutput:self->_videoOutput];
            else { [self->_session removeInput:input]; [self->_session commitConfiguration]; dispatch_async(dispatch_get_main_queue(), ^{self->_cameraStatus.text=@"Salida de cámara no disponible.";}); return; }
            self->_imageContext=[CIContext contextWithOptions:@{kCIContextUseSoftwareRenderer:@NO}];
            [self->_session commitConfiguration];
        }
        if (self.cameraGeneration != generation) return;
        [self->_session startRunning];
        // A stop queued while startRunning blocks must win over this start.
        if (self.cameraGeneration != generation) { [self->_session stopRunning]; return; }
        BOOL running = self->_session.isRunning;
        dispatch_async(dispatch_get_main_queue(), ^{
            if (self.cameraGeneration == generation) {
                self->_cameraStatus.text = running ? @"Cámara activa · vista previa local. USB sólo con autorización explícita." : @"No se pudo iniciar la cámara. Pulsa Iniciar para reintentar.";
                [self.view setNeedsLayout];
            }
        });
    });
}
- (void)switchCamera {
    NSUInteger generation=self.cameraGeneration;
    dispatch_async(_captureQueue, ^{
        if(self.cameraGeneration!=generation || !self->_session.running)return;
        AVCaptureDeviceInput *old=self->_session.inputs.firstObject;
        AVCaptureDevicePosition wanted=old.device.position==AVCaptureDevicePositionFront?AVCaptureDevicePositionBack:AVCaptureDevicePositionFront;
        AVCaptureDevice *chosen=nil;
        for(AVCaptureDevice *device in [AVCaptureDevice devicesWithMediaType:AVMediaTypeVideo]) if(device.position==wanted){chosen=device;break;}
        AVCaptureDeviceInput *next=chosen?[AVCaptureDeviceInput deviceInputWithDevice:chosen error:nil]:nil;
        if(!next)return;
        [self->_session beginConfiguration]; [self->_session removeInput:old];
        if([self->_session canAddInput:next]) [self->_session addInput:next];
        else if([self->_session canAddInput:old]) [self->_session addInput:old];
        [self->_session commitConfiguration];
    });
}
- (void)stopCamera {
    ++self.cameraGeneration; self.cameraUSBEnabled=NO; _cameraUSBRequested=NO; [_usbCameraToggle setTitle:@"Enviar cámara USB" forState:UIControlStateNormal];
    _cameraStatus.text = @"Cámara detenida."; [self.view setNeedsLayout];
    AVCaptureSession *session = _session;
    if (_captureQueue) dispatch_async(_captureQueue, ^{ [session stopRunning]; });
}
- (void)cameraInterrupted:(NSNotification *)notification {
    dispatch_async(dispatch_get_main_queue(), ^{
        [self stopCamera]; self->_cameraStatus.text = @"Cámara interrumpida. Pulsa Iniciar cámara cuando esté disponible.";
    });
}
- (void)cameraFailed:(NSNotification *)notification {
    NSError *error = notification.userInfo[AVCaptureSessionErrorKey];
    dispatch_async(dispatch_get_main_queue(), ^{
        [self stopCamera]; self->_cameraStatus.text = [NSString stringWithFormat:@"Error de cámara: %@", error.localizedDescription ?: @"vuelve a iniciar la cámara"];
    });
}
static void RVLE16(uint8_t *p, uint16_t value) { p[0] = value & 255; p[1] = value >> 8; }
static void RVLE32(uint8_t *p, uint32_t value) { RVLE16(p, value & 65535); RVLE16(p + 2, value >> 16); }
- (NSData *)toneData {
    const uint32_t rate = 22050, samples = rate * 5, bytes = samples * 2;
    NSMutableData *data = [NSMutableData dataWithLength:44 + bytes];
    uint8_t *p = data.mutableBytes;
    memcpy(p, "RIFF", 4); RVLE32(p + 4, 36 + bytes); memcpy(p + 8, "WAVEfmt ", 8);
    RVLE32(p + 16, 16); RVLE16(p + 20, 1); RVLE16(p + 22, 1);
    RVLE32(p + 24, rate); RVLE32(p + 28, rate * 2); RVLE16(p + 32, 2); RVLE16(p + 34, 16);
    memcpy(p + 36, "data", 4); RVLE32(p + 40, bytes);
    for (uint32_t i = 0; i < samples; i++) {
        double fade = fmin(1.0, fmin((double)i / 1102.0, (double)(samples - 1 - i) / 1102.0));
        int16_t value = (int16_t)(32767.0 * 0.12 * fade * sin(2.0 * M_PI * 440.0 * i / rate));
        RVLE16(p + 44 + i * 2, (uint16_t)value);
    }
    return data;
}
- (void)playTone {
    if (!_visible || [UIApplication sharedApplication].applicationState != UIApplicationStateActive) return;
    [self stopMicrophone]; [self stopUSBAudio]; [self stopTone];
    NSError *error = nil;
    AVAudioSession *audio = [AVAudioSession sharedInstance];
    if (![audio setCategory:AVAudioSessionCategoryPlayback error:&error] || ![audio setActive:YES error:&error]) {
        _audioStatus.text = [NSString stringWithFormat:@"No se pudo activar el audio: %@", error.localizedDescription];
        [audio setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil]; return;
    }
    _player = [[AVAudioPlayer alloc] initWithData:[self toneData] error:&error];
    _player.delegate = self;
    if (!_player || ![_player play]) {
        [self stopTone]; _audioStatus.text = [NSString stringWithFormat:@"No se pudo reproducir: %@", error.localizedDescription ?: @"audio no disponible"]; return;
    }
    _audioStatus.text = @"Tono suave de 440 Hz · 5 segundos. Ajusta el volumen del iPad.";
}
- (void)stopTone {
    [_player stop]; _player.delegate = nil; _player = nil;
    [[AVAudioSession sharedInstance] setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil];
    _audioStatus.text = @"Audio detenido.";
}
- (void)audioPlayerDidFinishPlaying:(AVAudioPlayer *)player successfully:(BOOL)flag {
    if (player != _player) return;
    [self stopTone]; _audioStatus.text = flag ? @"Tono terminado." : @"La reproducción se interrumpió.";
}
- (void)audioPlayerDecodeErrorDidOccur:(AVAudioPlayer *)player error:(NSError *)error {
    if (player != _player) return;
    [self stopTone]; _audioStatus.text = [NSString stringWithFormat:@"Error de audio: %@", error.localizedDescription ?: @"no se pudo reproducir"];
}
- (void)audioInterrupted:(NSNotification *)notification {
    if ([notification.userInfo[AVAudioSessionInterruptionTypeKey] unsignedIntegerValue] != AVAudioSessionInterruptionTypeBegan) return;
    dispatch_async(dispatch_get_main_queue(), ^{ [self stopMicrophone]; [self stopUSBAudio]; [self stopTone]; self->_audioStatus.text = @"Audio interrumpido. Pulsa Reproducir tono para reanudar."; });
}
- (void)startUSBAudio {
    if(!_visible || !_link.audioAccepted || [UIApplication sharedApplication].applicationState!=UIApplicationStateActive)return;
    [self stopMicrophone]; [self stopTone];
    _audioStatus.text=[_usbAudio start]?@"Audio USB habilitado · ajusta el volumen del iPad.":@"No se pudo activar audio USB.";
}
- (void)stopUSBAudio { [_usbAudio stop]; }
- (void)toggleUSBCamera {
    if(!_link.cameraAccepted)return;
    _cameraUSBRequested=!_cameraUSBRequested; self.cameraUSBEnabled=_cameraUSBRequested;
    [_usbCameraToggle setTitle:_cameraUSBRequested?@"Parar cámara USB":@"Enviar cámara USB" forState:UIControlStateNormal];
    if(_cameraUSBRequested) [self startCamera];
}
- (void)captureOutput:(AVCaptureOutput *)output didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer fromConnection:(AVCaptureConnection *)connection {
    if(!self.cameraUSBEnabled || !_link.cameraAccepted)return;
    CFTimeInterval now=CACurrentMediaTime(); if(now-_lastCameraFrame<0.1)return; _lastCameraFrame=now;
    @autoreleasepool {
        CVImageBufferRef pixel=CMSampleBufferGetImageBuffer(sampleBuffer); if(!pixel)return;
        CIImage *image=[CIImage imageWithCVPixelBuffer:pixel]; CGRect extent=image.extent;
        CGFloat scale=MIN(1.0,MIN(640.0/extent.size.width,480.0/extent.size.height));
        image=[image imageByApplyingTransform:CGAffineTransformMakeScale(scale,scale)];
        CGImageRef bitmap=[_imageContext createCGImage:image fromRect:image.extent]; if(!bitmap)return;
        NSData *jpeg=UIImageJPEGRepresentation([UIImage imageWithCGImage:bitmap],0.55); CGImageRelease(bitmap);
        if(self.cameraUSBEnabled && jpeg.length<=512*1024) [_link sendCamera:jpeg];
    }
}
- (BOOL)pointForTouch:(UITouch *)touch normalized:(CGPoint *)point {
    if(_display.hidden || !_display.image)return NO;
    CGSize bounds=_display.bounds.size; CGFloat scale=MIN(bounds.width/1024.0,bounds.height/768.0);
    CGRect rect=CGRectMake((bounds.width-1024*scale)/2,(bounds.height-768*scale)/2,1024*scale,768*scale);
    CGPoint p=[touch locationInView:_display];
    if(rect.size.width<=0 || rect.size.height<=0)return NO;
    BOOL inside=CGRectContainsPoint(rect,p);
    point->x=MAX(0,MIN(1,(p.x-rect.origin.x)/rect.size.width)); point->y=MAX(0,MIN(1,(p.y-rect.origin.y)/rect.size.height)); return inside;
}
- (void)sendChangedTouches:(NSSet<UITouch *> *)touches phase:(NSString *)phase {
    NSMutableArray *batch=[NSMutableArray new];
    for(UITouch *touch in touches){
        NSNumber *identifier=[_contacts objectForKey:touch];CGPoint point;
        if([phase isEqualToString:@"down"]){
            if(touch.view!=_display || ![self pointForTouch:touch normalized:&point])continue;
            NSUInteger next=1;while(next<=10 && _contactPoints[@(next)])next++;
            if(next>10)continue;identifier=@(next);[_contacts setObject:identifier forKey:touch];
        }else if(!identifier)continue;
        else {point=[_contactPoints[identifier] CGPointValue];[self pointForTouch:touch normalized:&point];}
        _contactPoints[identifier]=[NSValue valueWithCGPoint:point];
        [batch addObject:@{@"id":identifier,@"phase":phase,@"x":@(point.x),@"y":@(point.y)}];
        if([phase isEqualToString:@"up"] || [phase isEqualToString:@"cancel"]){[_contacts removeObjectForKey:touch];[_contactPoints removeObjectForKey:identifier];}
    }
    if(batch.count)[_link sendContacts:batch];
}
- (void)touchesBegan:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    if(_inputMode.selectedSegmentIndex!=0)return;
    if(_link.multitouchAccepted){[self sendChangedTouches:touches phase:@"down"];return;}
    if(_remoteTouch || !_link.touchAccepted)return;
    UITouch *touch=touches.anyObject;CGPoint p;if(touch.view!=_display || ![self pointForTouch:touch normalized:&p])return;
    _remoteTouch=touch;_lastTouch=p;[_link sendTouch:@"down" x:p.x y:p.y];
}
- (void)touchesMoved:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    if(_link.multitouchAccepted){[self sendChangedTouches:touches phase:@"move"];return;}
    if(!_remoteTouch || ![touches containsObject:_remoteTouch])return;
    CGPoint p=_lastTouch;[self pointForTouch:_remoteTouch normalized:&p];_lastTouch=p;[_link sendTouch:@"move" x:p.x y:p.y];
}
- (void)touchesEnded:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    if(_link.multitouchAccepted){[self sendChangedTouches:touches phase:@"up"];return;}
    if(!_remoteTouch || ![touches containsObject:_remoteTouch])return;
    CGPoint p=_lastTouch;[self pointForTouch:_remoteTouch normalized:&p];[_link sendTouch:@"up" x:p.x y:p.y];_remoteTouch=nil;
}
- (void)cancelRemoteTouch {
    NSMutableArray *batch=[NSMutableArray new];
    for(NSNumber *identifier in _contactPoints){CGPoint p=[_contactPoints[identifier] CGPointValue];[batch addObject:@{@"id":identifier,@"phase":@"cancel",@"x":@(p.x),@"y":@(p.y)}];}
    if(batch.count)[_link sendContacts:batch];[_contacts removeAllObjects];[_contactPoints removeAllObjects];
    if(_remoteTouch)[_link sendTouch:@"cancel" x:_lastTouch.x y:_lastTouch.y];_remoteTouch=nil;
}
- (void)touchesCancelled:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    if(_link.multitouchAccepted)[self sendChangedTouches:touches phase:@"cancel"];else [self cancelRemoteTouch];
}
- (void)inputModeChanged:(UISegmentedControl *)sender {
    [self cancelRemoteTouch];[self releaseKeyboard];[self releasePointer];
    [_keyboard resignFirstResponder];_remoteEditable=NO;[_link invalidateInputFocus];
    [self updateInputMode];
}
- (void)updateInputMode {
    BOOL keyboardMode=_link.keyboardAccepted && _link.inputModesAccepted;
    BOOL pointerMode=_link.pointerAccepted && _link.inputModesAccepted;
    [_inputMode setEnabled:keyboardMode forSegmentAtIndex:1];
    [_inputMode setEnabled:pointerMode forSegmentAtIndex:2];
    NSInteger mode=_inputMode.selectedSegmentIndex;
    if((mode==1 && !keyboardMode) || (mode==2 && !pointerMode)){
        mode=0;_inputMode.selectedSegmentIndex=0;
        [self releasePointer];[self releaseKeyboard];[_keyboard resignFirstResponder];
        _remoteEditable=NO;[_link invalidateInputFocus];
    }
    [_link sendInputMode:@[@"screen",@"keyboard",@"pointer"][mode]];
    _modeHint.text=mode==1?(_link.keyboardAccepted?@"Teclado para Windows":@"Conecta un host con teclado habilitado."):(_link.pointerAccepted?@"Desliza para mover · toque para clic\nDos dedos: scroll y clic derecho\nMantén pulsado para arrastrar":@"Conecta un host con mouse habilitado.");
    if(mode==1 && _link.keyboardAccepted && _visible)[_keyboard becomeFirstResponder];
    [self.view setNeedsLayout];
}
- (BOOL)textFieldShouldBeginEditing:(UITextField *)field {
    return _visible && _link.keyboardAccepted && (_inputMode.selectedSegmentIndex==1 || (_inputMode.selectedSegmentIndex==0 && _remoteEditable));
}
- (void)releasePointer {
    if(_pointerDragging)[_link sendPointer:@{@"dx":@0,@"dy":@0,@"button":@"left",@"phase":@"up"}];_pointerDragging=NO;
}
- (BOOL)pointerReady {return _inputMode.selectedSegmentIndex==2 && _link.pointerAccepted && _visible;}
- (void)pointerMove:(UIPanGestureRecognizer *)gesture {
    if(![self pointerReady])return;CGPoint p=[gesture translationInView:_trackpad];[gesture setTranslation:CGPointZero inView:_trackpad];
    if(gesture.state==UIGestureRecognizerStateChanged)[_link sendPointer:@{@"dx":@(p.x),@"dy":@(p.y)}];
}
- (void)pointerScroll:(UIPanGestureRecognizer *)gesture {
    if(![self pointerReady])return;CGPoint p=[gesture translationInView:_trackpad];[gesture setTranslation:CGPointZero inView:_trackpad];
    if(gesture.state==UIGestureRecognizerStateChanged)[_link sendPointer:@{@"dx":@0,@"dy":@0,@"scrollX":@(p.x),@"scrollY":@(p.y)}];
}
- (void)pointerClick:(NSString *)button {if(![self pointerReady])return;[_link sendPointer:@{@"dx":@0,@"dy":@0,@"button":button,@"phase":@"down"}];[_link sendPointer:@{@"dx":@0,@"dy":@0,@"button":button,@"phase":@"up"}];}
- (void)pointerTap:(UITapGestureRecognizer *)gesture {[self pointerClick:@"left"];}
- (void)pointerRightTap:(UITapGestureRecognizer *)gesture {[self pointerClick:@"right"];}
- (void)pointerDrag:(UILongPressGestureRecognizer *)gesture {
    if(![self pointerReady])return;CGPoint p=[gesture locationInView:_trackpad];
    if(gesture.state==UIGestureRecognizerStateBegan){_lastTouch=p;_pointerDragging=YES;[_link sendPointer:@{@"dx":@0,@"dy":@0,@"button":@"left",@"phase":@"down"}];}
    else if(gesture.state==UIGestureRecognizerStateChanged){[_link sendPointer:@{@"dx":@(p.x-_lastTouch.x),@"dy":@(p.y-_lastTouch.y)}];_lastTouch=p;}
    else [self releasePointer];
}
- (void)keyboardChanged:(UITextField *)field {
    if(field.markedTextRange || !field.text.length)return;
    NSString *text=[field.text copy];field.text=@"";
    // UTF-8 bound without splitting a composed character or losing IME marked text.
    __block NSMutableString *chunk=[NSMutableString new];
    [text enumerateSubstringsInRange:NSMakeRange(0,text.length) options:NSStringEnumerationByComposedCharacterSequences usingBlock:^(NSString *part,NSRange a,NSRange b,BOOL *stop){
        BOOL control=NO;for(NSUInteger i=0;i<part.length;i++){unichar c=[part characterAtIndex:i];if(c<32 || c==127)control=YES;}
        if(control){
            if(chunk.length)[self->_link sendKey:@{@"kind":@"text",@"text":chunk}];chunk=[NSMutableString new];
            if([part isEqualToString:@"\n"] || [part isEqualToString:@"\r"] || [part isEqualToString:@"\r\n"])[self->_link sendKey:@{@"kind":@"press",@"code":@"Enter"}];
            else if([part isEqualToString:@"\t"])[self->_link sendKey:@{@"kind":@"press",@"code":@"Tab"}];
            return;
        }
        if([[chunk stringByAppendingString:part] lengthOfBytesUsingEncoding:NSUTF8StringEncoding]>4096){if(chunk.length)[self->_link sendKey:@{@"kind":@"text",@"text":chunk}];chunk=[NSMutableString new];}
        if([part lengthOfBytesUsingEncoding:NSUTF8StringEncoding]<=4096)[chunk appendString:part];
    }];
    if(chunk.length)[_link sendKey:@{@"kind":@"text",@"text":chunk}];
}
- (BOOL)textFieldShouldReturn:(UITextField *)field {if(!field.markedTextRange)[_link sendKey:@{@"kind":@"press",@"code":@"Enter"}];return NO;}
- (void)controlKey:(UIButton *)button {
    NSString *code=button.currentTitle;
    if([code isEqualToString:@"Cerrar"]){_remoteEditable=NO;[_link invalidateInputFocus];[self releaseKeyboard];[_keyboard resignFirstResponder];return;}
    if([@[@"Control",@"Alt",@"Shift",@"Meta"] containsObject:code]){
        BOOL held=[_heldKeys containsObject:code];[_link sendKey:@{@"kind":held?@"up":@"down",@"code":code}];
        if(held)[_heldKeys removeObject:code];else [_heldKeys addObject:code];button.backgroundColor=held?[UIColor clearColor]:[UIColor blueColor];
    }else [_link sendKey:@{@"kind":@"press",@"code":code}];
}
- (void)releaseKeyboard {
    for(NSString *code in _heldKeys)[_link sendKey:@{@"kind":@"up",@"code":code}];[_heldKeys removeAllObjects];_keyboard.text=@"";
    for(UIView *view in _keyboardAccessory.subviews)if([view isKindOfClass:[UIButton class]])view.backgroundColor=[UIColor clearColor];
}
- (void)textFieldDidEndEditing:(UITextField *)field {[self releaseKeyboard];}
- (void)requestMicrophone {
    if(!_visible || !_link.microphoneAccepted || [UIApplication sharedApplication].applicationState!=UIApplicationStateActive)return;
    if([_micButton.currentTitle isEqualToString:@"Parar micrófono"]){[self stopMicrophone];return;}
    NSUInteger request=++_microphoneRequest;
    __weak DisplayViewController *weakSelf=self;
    [[AVAudioSession sharedInstance] requestRecordPermission:^(BOOL granted){dispatch_async(dispatch_get_main_queue(), ^{
        DisplayViewController *controller=weakSelf;
        if(!controller || controller->_microphoneRequest!=request || !controller->_visible || !controller->_link.microphoneAccepted || [UIApplication sharedApplication].applicationState!=UIApplicationStateActive)return;
        if(!granted){controller->_micStatus.text=@"Micrófono no autorizado. Revisa Ajustes > Privacidad.";return;}
        [controller stopUSBAudio];[controller stopTone];
        BOOL ok=[controller->_microphone start];controller->_micStatus.text=ok?@"Micrófono transmitiendo por USB · audio Windows detenido.":@"No se pudo iniciar el micrófono.";
        [controller->_micButton setTitle:ok?@"Parar micrófono":@"Enviar micrófono" forState:UIControlStateNormal];
    });}];
}
- (void)stopMicrophone {++_microphoneRequest;[_microphone stop];[_micButton setTitle:@"Enviar micrófono" forState:UIControlStateNormal];_micStatus.text=@"Micrófono apagado.";}
- (void)background:(NSNotification *)notification { [self stopCamera]; [self stopTone]; [self stopLink]; }
- (void)dealloc {
    [UIApplication sharedApplication].idleTimerDisabled=NO;
    [_microphone stop]; [_link stop]; [_usbAudio stop];
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    AVCaptureSession *session = _session;
    if (_captureQueue) dispatch_async(_captureQueue, ^{ [session stopRunning]; });
    [_player stop]; _player.delegate = nil;
    [[AVAudioSession sharedInstance] setActive:NO withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:nil];
}
- (BOOL)prefersStatusBarHidden { return YES; }
- (UIInterfaceOrientationMask)supportedInterfaceOrientations { return UIInterfaceOrientationMaskLandscape; }
@end
