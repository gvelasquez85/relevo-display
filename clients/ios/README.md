# Relevo Display: cliente nativo experimental 0.5

Destino iPad2/iOS9.3.5, arquitectura armv7, UIKit/Objective-C. Bundle propio `com.relevodisplay.client`, icono original, instalación independiente de Yam. No utiliza Safari.

## Ampliación 0.5 · 25 octubre 2026

Compilado realmente ARMv7/iOS 9.3.5 con SDK15.2. No firmado ni probado físicamente en esta revisión.

- Multitáctil directo: hasta diez contactos normalizados sobre la imagen, IDs estables durante cada contacto, fases down/move/up/cancel. Los bordes negros no inician contactos; se conserva el touch de un dedo si el host no negocia multitouch. Windows decide qué gestos interpreta cada aplicación; los gestos reservados del sistema iOS no se interceptan.
- Teclado nativo: texto confirmado Unicode (no se envía composición IME pendiente), retroceso, Enter y barra desplazable de controles/modificadores; botones KeyC/KeyV/KeyA permiten combinaciones con Control. Se liberan los modificadores al cerrar el teclado o terminar la sesión.
- Micrófono: apagado por defecto, permiso del sistema y botón explícito. AudioQueue captura PCM22050Hz/mono/16bits en paquetes de hasta20ms; cola de envío máximo tres paquetes pendientes, descarta exceso. No graba ni almacena audio. El host debe negociar microphone; recepción no equivale todavía a dispositivo de micrófono virtual de Windows.
- Para evitar eco y conflictos de AVAudioSession en esta prueba, micrófono y reproducción USB/tono son mutuamente excluyentes: activar uno detiene el otro. No se anuncia audio simultáneo bidireccional.
- Las capacidades multitouch/keyboard/microphone son opcionales. Los hosts v0.4 reciben solamente los campos de estado originales.
- Fondo/desconexión/parada detienen el micrófono y cancelan entrada. No se reactivan automáticamente.

## Implementado en fuente

- **Iniciar cámara / Detener cámara:** autorización explícita y vista previa local. No graba. Enviar cámaraUSB activa explícitamente el envío si el host negoció cámara.
- **Reproducir tono / Detener sonido:** tono440Hz de cinco segundos, nivel reducido y entrada/salida gradual. La prueba local no captura micrófono. El botón EscucharWindows habilita el canal de audio de Windows si se negoció.
- **Iniciar enlace USB / Detener enlace USB:** listener sólo127.0.0.1:52742, una conexión, protocolo propio RVL1. Handshake1024×768, JPEG validado con ImageIO antes de decodificar y una imagen pendiente. ACK indica asignación aUIImageView, no presentación física.
- Cámara y pantalla USB pueden funcionar simultáneamente en esta prueba; el desempeño combinado requiere validación. Salir/pasar al fondo detiene cámara, tono y conexión; no hay reinicio automático. Touch de un dedo compatible con hosts antiguos (multitáctil con host0.5), cámara USB hasta640×480/10FPS objetivo y audio PCM de Windows implementados para ensayo, con activación explícita y colas acotadas. No se ha validado físicamente.

## Estado real

Proyecto/plist/script revisados; pruebas del protocolo y sesiones del host ejecutadas en Mac con pares simulados. **El código Objective-C se compiló correctamente para ARMv7 con mínimo iOS9.3.5 y SDK15.2 oficial. Aún no está firmado ni instalado en el iPad.** El IPA sin firma requiere un certificado/perfil válido o una herramienta de firma compatible antes de instalar. El host Windows se ha compilado, pero no probado con dispositivo físico.

## Compilar con SDK oficial armv7

SDK15.2 oficial de Xcode13.2.1 ya extraído, con firma Apple y checksum del archivo verificados. No basta el Xcode actual ni Command Line Tools sin SDK iPhoneOS. Compilación comprobada con SDK15.2 y Apple clang21; esto no acredita todavía ejecución física.

```sh
bash build-with-sdk.sh /ruta/oficial/iPhoneOS15.2.sdk
```

Sólo produce app sin firma si compilación/enlace terminan correctamente. Incluye RVConnection, AVFoundation, ImageIO, iconos y familia iPad. Alternativa: abrir RelevoDisplay.xcodeproj en Xcode compatible con armv7 y deployment9.3.5. No instalar binarios sin firma en iOS estándar.

[Apple: versiones anteriores de Xcode, sin membresía de pago](https://developer.apple.com/xcode/resources/). Firma/perfil válido para el dispositivo se resuelven después de compilar; no se guardan credenciales en código.

## Prueba USB cuando la app esté instalada

1. Conectar/desbloquear iPad y confiar en Windows. Conservar Yam instalada; esta prueba usa la app **Relevo Display**, no Yam.
2. Abrir Relevo en iPad y pulsar **Iniciar enlace USB**.
3. En Windows ejecutar `Relevo-Native-Windows.exe`: patrón animado1024×768 durante20s. Requiere el servicio Apple/usbmux ya utilizado por el host actual.
4. Para captura real: `Relevo-Native-Windows.exe -mode desktop -duration 30s`; para pantalla virtual ya preparada: `-mode extend -duration 30s`. Este ejecutable no instala drivers. `-fps 60 -quality 80` solicita esos parámetros; no garantiza60FPS.
5. Verificar imagen, desconexión, parada, segundo plano y reinicio. No afirmar fluidez hasta probar el iPad. Para touch usar -mode desktop o extend y -touch; no disponible con patrón.

Añadir -audio para capturar la salida Windows y -camera para recibir cámara. Activar sus controles en iPad. Cámara entrega preview local y fuente opcional para OBS Virtual Camera; no dispositivo propio de webcam.

El host experimental es CLI separado del host gráfico estable0.10. Pantalla virtual reutiliza las políticas/driver existentes; esta prueba no sustituye el instalador final.

Actualización 25/10/2026: OBS queda descartado como dependencia del producto. El puente descrito arriba corresponde sólo al prototipo anterior. Diseño de reemplazo integrado, multitáctil y teclado: `../../docs/native-camera-touch-keyboard.md`. Aún no implementado.


## Input modes v0.6 (25 October 2026)

The client starts with the native keyboard hidden. In **Pantalla**, only a negotiated
`inputFocus` response following the most recent remote touch may open it. Windows
reports only an editable boolean, never a field value. Older hosts without this
capability leave automatic keyboard presentation disabled.

**Teclado** is an explicit dedicated input mode: it hides the received desktop and
opens the iOS keyboard when keyboard input is negotiated. **Mouse** hides the desktop
and exposes a relative trackpad: one-finger motion and tap, two-finger scroll and
secondary click, long press with movement to drag. Mode changes release contacts,
modifier keys and any drag button, dismiss the keyboard and invalidate old focus
responses. Dedicated selectors are disabled unless their input capability and `inputModes`
are negotiated. Frame 16 requests the host to suspend image production in the two
dedicated modes; one already in-flight image is still decoded and acknowledged.
Older hosts cannot enter these dedicated modes.

Wire extensions (all optional start booleans; explicitly echoed in ready only when
requested): `inputFocus`, `pointer`, `inputModes`. Type 14 host focus JSON is
`{"editable":true,"sequence":123}`, with the same nonzero uint32 in the frame
header and the latest type 4/11 gesture sequence. Type 15 pointer JSON always has
`dx` and `dy` relative point deltas; optionally `button` left/right and `phase`
down/up, or scrollX/scrollY point deltas. Type 16 is
`{"mode":"screen"}` / `keyboard` / `pointer`.

Physical acceptance checks still required: keyboard stays closed after launch,
ordinary desktop clicks and stale focus responses; opens on editable field tap;
closes on noneditable tap; dedicated modes stop host video; mouse drag is released
on mode change, disconnect and background; iOS 9 native keyboard Unicode and IME
composition survive switching. ARMv7 compilation is a build check, not physical QA.
