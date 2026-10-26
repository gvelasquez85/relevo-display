# Audio y webcam: viabilidad de prueba

Renzo Raines · 24 de octubre de 2026. Encargo coordinado por Hannibal, run `relevo-media-20261024`. Entorno objetivo: Windows 11, iPad 2, iOS **9.3.5**, receptor Yam existente. Investigación documental; ninguna prueba de cámara, micrófono, altavoz ni Bluetooth realizada en el dispositivo.

## Decisión propuesta

La prueba inmediata más viable es **audio de Windows al iPad mediante una página local de Safari**, independiente de Yam y por red local. La prueba de tono sirve para validar transporte y salida audible; un archivo finito no mide latencia de audio en vivo. No prometer funcionamiento simultáneo con el monitor: cambiar de Safari a Yam puede suspender el audio. La webcam necesita un cliente nativo compatible ya instalado o nuestro propio cliente firmado; Safari 9 no resuelve la captura en vivo.

## Evidencia y límites

| Capacidad | Hecho documentado | Consecuencia para esta prueba |
|---|---|---|
| Audio recibido en Safari | Apple documenta Web Audio desde iOS 6, carga binaria con XMLHttpRequest, buffers, programación y activación mediante toque. | Es razonable ensayar en iOS 9.3.5. No equivale a validación en nuestro iPad. |
| Cámara/micrófono web | WebKit anunció captura y WebRTC para Safari de iOS 11 el 7/6/2017. | No diseñar webcam en vivo basada en getUserMedia para iOS 9.3.5. Un selector de archivo/foto no sustituye un flujo en vivo. |
| Audio Windows | WASAPI loopback captura la mezcla de un endpoint de reproducción en modo compartido, sin exigir un dispositivo «Stereo Mix». | Capturar salida existente no necesita instalar un driver virtual. Esto tampoco crea una salida llamada Relevo en Configuración de sonido. |
| Cámara virtual Windows | MFCreateVirtualCamera requiere Windows build 22000 y una fuente de cámara software identificada por sourceId; respeta controles de privacidad. | Windows 11 ofrece una vía de publicación, pero hay que implementar la fuente y recibir imágenes reales del iPad. No basta registrar un nombre de dispositivo. |
| Cámara nativa iOS | AVFoundation proporciona AVCaptureSession, dispositivos y salidas; permite consultar presets disponibles. | Camino para cliente propio. Empezar con una cámara y resolución consultada, no asumir captura doble o 720p frontal. |
| Bluetooth | Apple describe la salida Web Audio hacia el dispositivo de audio predeterminado, que puede ser altavoz, auriculares o headset Bluetooth. | Si el accesorio logra emparejarse y iOS lo selecciona, ensayar la ruta. No demuestra compatibilidad de un modelo de AirPods ni acceso Windows al adaptador Bluetooth. |

No hay evidencia retenida de canal de audio/cámara en el protocolo Yam analizado. No añadir paquetes especulativos al flujo de pantalla ni anunciar estas funciones como disponibles. `docs/ideas-futuras-ipad.md` conserva referencias históricas a iOS7; la versión confirmada por German y vigente es 9.3.5.

## Experimento mínimo de audio

Propuesta técnica, pendiente de implementación y prueba:

1. Modo explícito de laboratorio, sin inicio automático. Primero tono de prueba; después captura WASAPI del endpoint elegido. Detener libera captura y servidor.
2. Servidor local con URL que incluya token aleatorio de sesión. LAN privada, sin cloud, sin guardar audio ni abrir puertos de router. Para HTTP local, advertir concretamente que el audio viaja sin cifrar; no ofrecerlo como opción para redes compartidas. No cambiar firewall automáticamente.
3. Página ES5: `window.AudioContext || window.webkitAudioContext`, XMLHttpRequest y callbacks. No depender de fetch, async/await, AudioWorklet o WebRTC. Botón «Activar audio» inicia contexto desde gesto humano.
4. PCM estéreo o mono, bloques independientes con secuencia y frecuencia explícita; convertir a Float32 y programar AudioBufferSourceNode. Cola con límite temporal, descartar acumulación, mostrar cortes. Evitar un WAV infinito y evitar decodeAudioData sobre fragmentos incompletos.
5. Medir duración de cola programada, bloques descartados y cortes; estos contadores no son latencia acústica real. Una prueba filmada de señal y sonido mide retardo perceptible.
6. Probar altavoz, auriculares y accesorio Bluetooth disponible por separado. Probar suspensión al cambiar a Yam, bloquear pantalla y reconectar red. No prometer audio con pantalla extendida hasta comprobar esa transición.

Criterio propuesto para continuar: diez minutos de reproducción comprensible, parada inmediata, ausencia de crecimiento sostenido de cola, y recuperación tras interrupción. Umbral inicial de laboratorio para cola: objetivo 150–300 ms, límite 500 ms; son **objetivos**, no resultados ni latencia garantizada. Si Safari9 no permite una reproducción estable, cerrar esta vía y esperar cliente nativo. Para una salida exclusiva seleccionable por aplicación, evaluar un driver de audio aparte más adelante.

## Webcam: siguiente experimento, con dependencia explícita

Cliente nativo iOS 9.3.5 con AVCaptureVideoDataOutput y vista previa, selector frontal/trasera, botón de iniciar y detener, indicador visible, sin grabación. Primer receptor Windows muestra vista previa; sólo después integrar fuente Media Foundation como webcam en Windows11. Validar cámara en al menos una aplicación de videollamada antes de afirmar interoperabilidad. SDK/firma/instalación siguen pendientes; no hay ejecutable de webcam para este iPad todavía. No instalar ni recomendar una app comercial actual sin verificar que ofrece binario armv7 y descarga utilizable en iOS9.3.5.

## Fuentes primarias consultadas el 24/10/2026

- [Apple: Playing Sounds with the Web Audio API](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/Using_HTML5_Audio_Video/PlayingandSynthesizingSounds/PlayingandSynthesizingSounds.html), archivo histórico; secciones Setting Up, Fetching and Buffering y Playing Sounds.
- [WebKit: Announcing WebRTC and Media Capture](https://webkit.org/blog/7726/announcing-webrtc-and-media-capture/), 7/6/2017.
- [Microsoft: Loopback Recording](https://learn.microsoft.com/en-us/windows/win32/coreaudio/loopback-recording), actualizado 16/4/2025; captura compartida, eventos desde Windows10 1703 y limitaciones de contenido protegido.
- [Microsoft: MFCreateVirtualCamera](https://learn.microsoft.com/en-us/windows/win32/api/mfvirtualcamera/nf-mfvirtualcamera-mfcreatevirtualcamera), requisitos y permisos.
- [Apple: Still and Video Media Capture](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/AVFoundationPG/Articles/04_MediaCapture.html), archivo histórico de AVFoundation.
- [WebKit bug 220879](https://bugs.webkit.org/show_bug.cgi?id=220879), discusión del comportamiento de Web Audio en segundo plano, 2021. Evidencia de restricción del motor, **no prueba específica de iOS9.3.5**; requiere comprobación física.

## Prueba alternativa inmediata de cámara: fotografía manual

Apple documenta `<input type="file">` desde iOS6 en [Creating Compatible Web Content](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/CreatingContentforSafarioniPhone/CreatingContentforSafarioniPhone.html). Permite proponer selección explícita y envío de una foto al servidor local; **no constituye cámara en directo**. Usar campo visible `accept="image/*"` y formulario `multipart/form-data` como base sin dependencia de APIs modernas. El atributo `capture="camera"` puede ensayarse como indicación, pero no se ha encontrado garantía oficial de que fuerce cámara en iOS9.

El [reporte WebKit 157507, de 2016](https://bugs.webkit.org/show_bug.cgi?id=157507) describe que `accept="image/*"` invoca la cámara en iOS9 y también denuncia fallo de subida. Es evidencia directa de un reporte de usuario en el tracker del motor, no confirmación de que nuestro flujo funcione en 9.3.5. Mantener selección de foto existente como alternativa y validar ambos recorridos. Mostrar foto recibida sólo en memoria con límite de bytes/dimensiones y botón de borrar; no asumir que MIME o extensión prueban contenido. Criterio de éxito: fotografía tomada o elegida conscientemente aparece en Windows, cancelación no envía nada y detener laboratorio borra la imagen. Esto valida transporte inverso y una interacción de cámara puntual, sin instalar app ni anunciar webcam.

## Entrega experimental implementada: Relevo Lab 0.1

Ejecutable separado `dist/Relevo-Lab-Windows.exe`: tono WAV finito servido desde Windows, detector de APIs y foto manual del iPad al servidor. No incluye captura WASAPI, audio continuo ni webcam virtual. Los pasos de PCM en bloques descritos arriba siguen siendo propuesta. La interfaz del laboratorio explica que Safari en primer plano puede pausar Yam.

Doble clic y Enter inicia LAN por elección explícita; abre la página local en Windows y anuncia enlaces privados para Safari. Sin inicio automático ni instalación de drivers. La foto JPEG/PNG tiene límite de 5 MiB para toda la petición y 4 millones de píxeles, se convierte a JPEG sin metadatos y sólo se conserva la última en RAM. El token temporal protege todas las rutas, pero HTTP no cifra la red local. Compatibilidad física con Safari9.3.5 y funcionamiento Windows pendientes de prueba.
