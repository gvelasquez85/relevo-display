> Actualización posterior del usuario, 24 de octubre de 2026: el dispositivo real tiene **iOS 9.3.5**, y ese mínimo es suficiente. Las referencias siguientes a iOS 7 y a soporte obligatorio de iOS 5.1.1 describen el supuesto anterior y quedan sustituidas para la siguiente implementación.

# Relevo Display: requisitos del cliente propio

24 de octubre de 2026. Análisis de requisitos; no implementación ni prueba en iOS. Responsable del documento: worker client_requirements, coordinación Hannibal, ejecución relevo-phase2-analysis-20261024.

## Punto de partida y resultado buscado

German confirma que Relevo 0.6 muestra una pantalla reconocida por Windows 11 en su iPad 2 con iOS 7 y que la fluidez mejoró mucho; todavía percibe lag. Es evidencia del usuario sobre su equipo, no un benchmark. La fase 1 queda pendiente de mejorar y medir fluidez; la siguiente fase debe reemplazar el receptor del iPad para que Relevo sea independiente del producto original.

Equipo de referencia: iPad 2, iOS 7, Windows 11, USB, imagen 1024×768 y relación 4:3. El cliente propio deberá mostrar el escritorio y enviar entrada táctil. Linux y Raspberry Pi, incluida sesión sin monitor físico, son etapas posteriores. iPhone queda aplazado. Soporte iOS 5.1.1 es un objetivo adicional para el primer iPad, que requiere pruebas propias y no se deduce del éxito del iPad 2.

## Independencia real y procedencia

La inspección de cmd e internal no encontró un comprobador propio de licencias de Yam; sí existen textos, identificadores de interoperabilidad, migración y respaldos heredados. No se ha auditado todo el comportamiento del receptor propietario ni se afirma nada sobre sus términos. El paquete sigue necesitando esa app instalada en el iPad: cambiar textos del servidor no elimina esa dependencia.

La independencia funcional exige un cliente iOS creado por nosotros, su protocolo documentado y una instalación demostrada. No copiar código, recursos, marca ni mecanismos de licencia del tercero. Las observaciones del tráfico y del comportamiento son evidencia para interoperabilidad, no una conclusión jurídica sobre autorización de distribución. No se propone eludir licencias ni modificar el binario propietario.

Conservar el historial técnico y la procedencia donde corresponde. Sacar referencias heredadas de las superficies de producto de Relevo y, cuando sea viable, mantener la compatibilidad antigua como adaptador separado. No borrar avisos de terceros: hoy el proyecto usa Go, howett.net/plist, godbus/dbus y distribuye el controlador MTTVDD con su licencia MIT. THIRD_PARTY_NOTICES.txt y las licencias de los componentes deben acompañar las distribuciones que los incluyan. El nuevo receptor puede eliminar la dependencia de Yam, pero no la de esos componentes. El servicio USB de Apple continúa siendo una dependencia del host Windows mientras se use la integración actual; su distribución requiere una decisión específica, no asumir que puede incorporarse al instalador.

## Arquitectura propuesta

- Cliente legacy Objective-C/UIKit de 32 bits, con APIs disponibles en el sistema objetivo y gestión explícita del consumo de memoria. Evitar requerir SwiftUI, Metal o frameworks modernos en la ruta básica.
- Núcleo pequeño en C para framing, negociación, límites y estado de sesión, reutilizable donde aporte valor. El servidor Go implementa el mismo contrato; no requiere trasladar todo el servidor a C.
- Build moderno separado o targets separados con detección de capacidades. Una optimización moderna no eleva silenciosamente el mínimo del cliente legacy.
- Protocolo propio Relevo v1, separado del adaptador de interoperabilidad existente. La nueva app no debe heredar nombres, puertos ni envoltorios privados sólo por comodidad.
- USB: el cliente abre un socket local y el host conecta mediante el servicio de multiplexación de dispositivos; la aplicación no obtiene acceso USB bruto ni control general de iOS. Validar emparejamiento, acceso al puerto y reconexión en iOS 7 real. Wi-Fi es futuro y necesitará autenticación y cifrado compatibles con ambos extremos; no abrir un receptor anónimo en la red.

## Contrato mínimo de protocolo v1

Documentar antes de implementar: versión, longitud y tipo de mensaje; tamaño máximo negociado; orden de bytes; identificador de sesión; tratamiento de mensajes desconocidos; timeouts y cierre. Separar control, vídeo y entrada para evitar que una cola de imágenes retrase touch o cancelación.

Handshake: capacidades del dispositivo, tamaño de pantalla útil, orientación, formatos/códecs admitidos y límites; selección explícita del host y confirmación. No asumir que cualquier resolución solicitada está disponible en el monitor virtual.

Vídeo: dimensiones, códec, número de secuencia, timestamp monotónico, rectángulo visible y, en H.264, formato exacto de muestras, perfil/level, parameter sets, indicación IDR y petición de nuevo IDR. El cliente debe confirmar recepción/presentación cuando pueda medirlas. Los relojes de host y cliente no se restan como si estuvieran sincronizados.

Entrada: touch down/move/up/cancel, identificador del contacto y coordenadas normalizadas del contenido, orientación y letterboxing; ratón, rueda y teclado como capacidades separadas. El host aplica la transformación al monitor extendido concreto. Versionar gestos y distinguir eventos raw de acciones interpretadas para evitar doble clic o doble scroll. Rechazar coordenadas inválidas y liberar botones/teclas al perder conexión.

## Códec y rendimiento

JPEG es la primera ruta implementable y contrastable con el servidor actual. H.264 Baseline de baja latencia, sin B-frames, es candidato experimental, no mejora asegurada. Primero localizar captura, conversión, codificación, escritura, decodificación y presentación; aumentar FPS solicitado no prueba refresco visible.

La evaluación existente documenta que AVSampleBufferDisplayLayer y las funciones públicas VTDecompressionSessionCreate/DecodeFrame están disponibles desde iOS 8. Por tanto no son una solución directa para iOS 7/5.1.1. Ver [evaluación del códec](codec-evaluation-2026-10-24.md), [AVSampleBufferDisplayLayer](https://developer.apple.com/documentation/avfoundation/avsamplebufferdisplaylayer) y [VTDecompressionSessionCreate](https://developer.apple.com/documentation/videotoolbox/vtdecompressionsessioncreate(allocator:formatdescription:decoderspecification:imagebufferattributes:outputcallback:decompressionsessionout:)). El soporte H.264 publicado del hardware no demuestra una API pública adecuada para un stream interactivo propio en esos sistemas.

Investigar reproducción con contenedor compatible o decodificación software mediante prototipo medido; AVPlayer/HLS convencional puede introducir buffering. No adoptar APIs privadas como requisito. Mantener JPEG mientras no se demuestre otra ruta. Todo codec externo añadido requiere revisar licencia, tamaño y consumo antes de empaquetar.

Objetivos propuestos para acordar: 25 FPS visibles sostenidos en movimiento a 1024×768, latencia visible p95 ≤150 ms y ausencia de errores de decodificación en comparación repetida. No están alcanzados ni medidos. Limitar colas, conservar la imagen más reciente antes de codificar y usar backpressure; no descartar P-frames arbitrariamente. Registrar percentiles y FPS de cada etapa, CPU y memoria, además de observación de cursor/scroll. Una mejora de codec sólo se acepta si reduce latencia sin empeorar estabilidad o legibilidad.

## Funciones y calidad del primer cliente

| Requisito | Criterio propuesto de aceptación |
|---|---|
| Imagen y proporción | 1024×768 sin deformación; orientación y letterbox coherentes con entrada |
| Touch básico | Clic, arrastre, clic derecho y scroll en la pantalla extendida correcta; cancelación libera estado |
| Conexión USB | Estado visible, errores útiles y reconexión sin reiniciar Windows ni reinstalar controlador |
| Suspensión y UAC | Pausa/reanudación explícitas; no capturar escritorio seguro ni enviar entrada a él |
| Pantalla y energía | Evitar reposo sólo durante sesión; restaurar comportamiento al salir; medir temperatura/batería |
| Experiencia legacy | Controles legibles, VoiceOver donde corresponda y salida de sesión accesible |
| Privacidad | Sin almacenamiento de pantallas ni pulsaciones por defecto; informes sólo de metadatos y exportación voluntaria |
| Robustez | Límites de memoria, tamaños y tiempos; parser probado con entradas truncadas/inválidas; sesión se recupera o cierra limpiamente |

El host conserva GUI, bandeja, elección de resolución/calidad, perfiles, controlador y restauración. La app iOS debe empezar simple: conectar, mostrar pantalla, entrada y ajustes esenciales. Cámara, audio, AirPods/Bluetooth y control de otras apps no son requisitos del primer cliente; figuran como hipótesis en ideas-futuras-ipad.md y requieren APIs/permisos y experimentos separados. No asumir control arbitrario de aplicaciones de iOS.

## Primer hito que demuestra viabilidad

1. Preparar toolchain reproducible capaz de generar un ejecutable armv7 válido para el iPad 2/iOS 7. Registrar SDK, compilador, deployment target, arquitectura y dependencias efectivas.
2. Firmar e instalar una app mínima en ese iPad físico con una vía permitida, documentando certificado/provisión y cualquier caducidad. El estudio de instalación cubre alternativas pagadas y gratuitas; ninguna está validada aquí. No depender de TestFlight moderno para ese sistema.
3. Establecer sesión USB propia, mostrar una imagen 1024×768 y recibir un touch con transformación comprobable. Guardar log de metadatos y evidencia visual explícita; registrar versión de iOS y arquitectura sin publicar identificadores privados.
4. Después transmitir escritorio, activar touch y medir latencia con el mismo Windows 11. Comparar con Relevo compatible; conservar el paquete funcional anterior durante la transición.
5. Abrir un hito independiente para iOS 5.1.1 en primer iPad: instalación, arranque, socket, imagen, touch, estabilidad y rendimiento. No marcarlo soportado por compilar con un número mínimo ni por probar sólo en iOS 7.

Salida de este análisis: requisitos y puertas de validación. No se ha construido ni instalado el cliente propio; no se ha validado H.264 en él ni una distribución legacy. La eliminación de referencias de marca no sustituye estas pruebas.
