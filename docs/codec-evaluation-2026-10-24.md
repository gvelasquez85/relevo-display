# Relevo Display: evaluación del códec para iPad 2

24 octubre 2026. Responsable: Renzo Raines (kipp), investigación acotada de sólo lectura, ejecución screen-identity-codec-20261024. No se intervino la sesión del dispositivo. Este documento conserva nombres del producto analizado únicamente como referencia de interoperabilidad.

## Decisión propuesta

Investigar **H.264 Baseline, 8 bits, 4:2:0, sin B-frames y con latencia baja**, empezando en 1024×768 a 20/30 FPS. Mantener JPEG como ruta estable mientras se valida la recepción real. No sustituir el códec por HEVC o AV1 en el iPad 2: no hay soporte demostrado en ese dispositivo o cliente y añadir decodificación por software contradice el objetivo de fluidez.

Es una hipótesis de mejora, no una mejora medida: los cuadros predictivos de H.264 pueden reducir tráfico y trabajo de decodificación frente a JPEG independiente; pueden también introducir colas y retardos si el encoder se configura mal. Cambiar el nombre de marca o activar un flag H264 no implementa un encoder ni demuestra compatibilidad.

## Hechos y evidencia reproducible

- Apple documenta pantalla 1024×768 y reproducción H.264 hasta 1080p30, High Profile Level 4.1 para iPad 2. Eso demuestra capacidad multimedia publicada; no demuestra el comportamiento del cliente instalado ni disponibilidad de una API de decodificación concreta en iOS 7. [Especificaciones oficiales](https://support.apple.com/en-us/111990), consultadas 24 octubre 2026.
- `analysis/protocol-encoded.json`, `normalized_result.pseudocode`, función `-[OutputDestination sendEncodedData:channel:peer:]` en `0x10000fa28`, llama al empaquetador H.264 con tipo **107 / 0x6b**; para sincronización establece tag 100, en otros cuadros tag 0. La ruta JPEG usa108.
- `analysis/protocol-h264payload.json`, función `0x10000fcd8`, evidencia `ev_c18128b630bfaddf00c7a7feb8da9abeff9a35195540f5e72b7236fa7d3e3f4a`: lee CMSampleBuffer, NotSync, CMBlockBuffer y parameter sets H.264 índices0/1 (SPS/PPS). El ensamblado confirma un envoltorio propio; no se debe mandar Annex B estándar directamente.
- `analysis/protocol-trial-vtsetup.json`, `-[AppleVTCompressor setupCompressor:]` en `0x100056dd4`, evidencia `ev_cd711b309710449cac0037c61ab2681aabe96a5492e442cd8051fe6fa198d3f4`: solicita encoder acelerado para `avc1`, establece `RealTime=true`, `AllowFrameReordering=false`, configura bitrate medio y límite de datos, e incluye perfiles Baseline/Main/High. En algunas ramas configura intervalo de keyframe de2 segundos y bitrates medios4000/5000/6000×1000; no se afirma que esas ramas sean las activas en el iPad conectado.
- `analysis/protocol-trial-codec.json`, evidencia `ev_92c8301a9745cb43296efe1b2caeb5b0040c637bc02c1f45b982a124d66a1655`: distingue JPEG=`0x6a706567` y AVC=`0x61766331`, consulta bH264HWCodec, tamaño y peerHardwareString con rutas iPad2/iPad3. El pseudocódigo tiene condiciones contradictorias por pérdida de flujo/registros; no tomarlo como código fuente exacto.

Para repetir inspección sin imprimir el dossier completo: cargar JSON en Python y seleccionar `normalized_result.pseudocode` y `normalized_result.assembly`. Las direcciones y SHA del ejecutable analizado constan dentro de cada JSON.

## Envoltorio H.264 observado

Los siguientes offsets proceden del ensamblado arm64 del servidor Mac. Endian interior little-endian; la cabecera PeerTalk exterior conserva big-endian.

Cuadro no sincronizado, bloque de muestra M:

```
u32LE(len(M)+5), u32LE(1), 00 00 00 01, 21, M
```

Cuadro sincronizado, parameter sets S/P y bloque M:

```
u32LE(len(S)+len(P)+len(M)+15), u32LE(1),
00 00 00 01, 27, S,
00 00 00 01, 28, P,
00 00 00 01, 25, M
```

Los bytes27/28/25/21 son escrituras explícitas. **No se ha demostrado** si el receptor trata esos bytes como marcadores propios, cabeceras NAL sintéticas u otro formato; tampoco si S/P incluyen cabecera NAL original ni la distribución AVCC de M que acepta iOS. CMSampleBuffer puede contener más de una NAL. Copiar estas fórmulas sobre cualquier salida de FFmpeg o Media Foundation puede duplicar cabeceras y producir ErrorDecoding. Falta contraste con datos del emisor original o experimento acotado.

## Implementación recomendada después de comprobar framing

Windows11: Media Foundation MFT, fuente BGRA convertida a NV12, salida MFVideoFormat_H264, perfil Baseline, tasa inicial4Mbps ajustable y `CODECAPI_AVLowLatencyMode=VT_BOOL(true)` donde el encoder lo soporte. Enumerar y comprobar capacidades del encoder instalado; hardware no está garantizado. Mantener alternativa software y no hacer depender la instalación de FFmpeg externo. Microsoft documenta perfiles, formatos de entrada, sequence header y propiedades de bitrate/GOP; el modo de latencia baja evita retardo por reordenación, no garantiza una latencia extremo a extremo. [Encoder H.264](https://learn.microsoft.com/en-us/windows/win32/medfound/h-264-video-encoder), [modo de latencia baja](https://learn.microsoft.com/en-us/windows/win32/medfound/codecapi-avlowlatencymode), consultados24octubre2026.

Linux: backend GStreamer con negociación de encoder disponible. Evaluar `x264enc tune=zerolatency`, sin B-frames, cola de captura de tamaño1 y descarte de imágenes antiguas **antes de codificar**. No descartar arbitrariamente P-frames ya codificados: se romperían las referencias. El perfil se fija mediante caps downstream. [Documentación oficial x264enc](https://gstreamer.freedesktop.org/documentation/x264/index.html), consultada24octubre2026. Evaluar dependencias/licencia antes de empaquetar esta implementación.

RaspberryPi, etapa futura: detectar modelo y capacidades V4L2; no asumir aceleración homogénea. La documentación oficial distingue encoder hardware donde existe y software en Pi5; sus comandos de cámara son evidencia de disponibilidad, **no sustituyen captura de escritorio**. [Documentación RaspberryPi](https://www.raspberrypi.com/documentation/computers/camera_software.html), sección Low latency video with Raspberry Pi5 y ejemplos GStreamer, consultada24octubre2026.

Cliente propio: separar negociación de códec del transporteUSB, publicar dimensiones, perfil/level, FPS, timestamps y posibilidad de solicitar IDR. JPEG permanece como fallback. La ruta de decodificación/instalación propia para iOS5/7 requiere investigación independiente: soporte multimedia del hardware no equivale a API pública compatible con esos sistemas.

## Experimento y criterio de aceptación propuestos

1. Instrumentar captura, conversión, compresión y escritura por separado; sólo metadatos, sin contenido del escritorio ni identificadores de dispositivo. Registrar FPS solicitado, capturado, enviado y tiempo p50/p95 por etapa. El servidor no conoce FPS mostrado sin confirmación del receptor.
2. Validar encapsulado con una secuencia sintética pequeña, seguida de IDR/P-frames, 1024×768, con límites de tiempo y restauración de sesión. No sustituir el modo estable por defecto. Aceptar primero60segundos sin ErrorDecoding, imagen correcta y recuperación mediante IDR tras reinicio de stream.
3. Comparación JPEG/H264 en el mismo Windows11/iPad2/iOS7 con escritorio estático, cursor, scroll y vídeo; mínimo3repeticiones de60segundos por condición. Medir tráfico, CPU y etapas del servidor; grabación externa sincronizada para latencia visible. No confundir FPS enviados con refresco en iPad.
4. Umbrales propuestos para aprobar H264 por defecto: al menos25FPS visibles en movimiento a1024×768, latencia visible p95≤150ms, cero errores de decodificación y menor tráfico que JPEG a legibilidad comparable. Son objetivos de experimento por acordar, no métricas obtenidas. Si no mejora fluidez/latencia o empeora estabilidad, conservar JPEG y localizar el cuello de botella antes de aumentar complejidad.

## No probado

No se capturaron paquetes H264 reales; no se ejecutó encoderWindows/Linux/Pi; no hubo benchmark comparativo ni prueba de decodificaciónH264 propia en el iPad. La investigación justifica una prueba H264 de baja latencia, no declarar implementada la mejora.

## Barrera del cliente propio en iOS 5/7

Consulta oficial24octubre2026 mediante JSON de documentaciónApple: `AVSampleBufferDisplayLayer` se introduce en iOS8; `VTDecompressionSessionCreate` y `VTDecompressionSessionDecodeFrame` también exponen disponibilidad pública desde iOS8. El tipo `VTDecompressionSession` por sí solo figura desde iOS6; **no usar esa fecha del tipo para afirmar que las funciones de creación/decodificación funcionan públicamente en iOS7**.

Fuentes: [AVSampleBufferDisplayLayer](https://developer.apple.com/documentation/avfoundation/avsamplebufferdisplaylayer), [VTDecompressionSessionCreate](https://developer.apple.com/documentation/videotoolbox/vtdecompressionsessioncreate(allocator:formatdescription:decoderspecification:imagebufferattributes:outputcallback:decompressionsessionout:)), [DecodeFrame](https://developer.apple.com/documentation/videotoolbox/vtdecompressionsessiondecodeframe(_:samplebuffer:flags:framerefcon:infoflagsout:)). Comprobaciónreproducible: descargar el recurso JSON correspondiente en `https://developer.apple.com/tutorials/data/documentation/` y leer `metadata.platforms[].introducedAt`.

`AVPlayer` está documentado desdeiOS4, pero es un reproductor de activos/streams, no prueba de decodificación de NAL arbitrarias con baja latencia. Se puede experimentar un flujoH264 con contenedor compatible y servidorlocal/HLS, midiendo buffering; no prometer latencia interactiva mediante HLS convencional. Otra alternativa es decodificaciónsoftware, cuyo coste en A5 debe medirse, y conservar JPEG como clientepropioinicial. No recomendar APIsprivadas ni asumir que el éxito del receptororiginal hace pública su estrategia.

La optimización del emisorcompatible puede validarse antes de resolver el clienteiOSpropio; son dos entregables distintos de Relevo Display.
