# Relevo Display 0.9: comparación con la lista de Yam Display

24 de octubre de 2026. Alcance actual: **Windows 11 + iPad 2 con iOS 9.3.5 + app Yam instalada**. La lista suministrada por German es la referencia de comparación, no prueba de que cada función esté disponible en la versión instalada. Cliente iOS propio, Xcode, Raspberry Pi e iPhone quedan pausados en este encargo.

“Implementado” describe código y pruebas automáticas; no sustituye una prueba real en Windows. German confirmó imagen/monitor reconocido y mejora de fluidez en versiones previas. **El paquete 0.7 todavía requiere su prueba física.**

## Pantalla y preferencias

| Función de referencia | Relevo Windows | Alcance o trabajo pendiente |
|---|---|---|
| Conexión USB | Funciona según prueba del usuario | Conserva servicio Apple y receptor instalado; todavía no es cliente propio |
| Pantalla extendida | Implementada; reconocida por Windows en pruebas del usuario | MTTVDD/IddCx; restauración del estado previo |
| Duplicar la pantalla principal | Añadida a la GUI en 0.7 | Captura la imagen principal y la adapta al iPad; no cambia Windows al modo de topología “Duplicar” |
| Resoluciones | Parcial | Presets 4:3 para iPad2:1024×768,1280×960,1600×1200,2048×1536; no detección dinámica de todos los dispositivos |
| 30/60 FPS | Parcial | Solicitudes10/15/20/30/60FPS; ni garantía de que el iPad presente tantos frames. Los60Hz del driver no equivalen a60FPS de vídeo |
| Izquierda/derecha/arriba/abajo | Añadido en0.7 | Sólo extensión; queda fuera del borde de los monitores existentes, con restauración segura |
| Abrir ajustes de pantallas | Implementado | Abre configuración deWindows; detener antes de cambios externos |
| Lanzar al iniciar sesión | Pendiente | Debe ser preferencia explícita del usuario, sin iniciar transmisión automáticamente |
| Actualización automática | Pendiente | Necesita publicación, firma/verificación, recuperación y canal de actualización; no basta una casilla |
| Evitar reposo de pantalla | Añadido en0.7 | Opcional mientras dura la sesión; no altera el plan de energía permanente |
| Cursor del sistema | Parcial | Se incorpora a la imagen con hotspot/escala; falta elegir cursor local del receptor/tamaño alternativo |
| Zoom de la app principal | Pendiente | No se implementó control Windows; depende además del gesto/capacidad de la edición del receptor |
| Touch Bar | No equivalente directo | Función macOS; un panel de accionesWindows sería otro diseño |
| Extender medianteAirPlay | No aplica al backendWindows | Se conserva monitor virtualIddCx; no implementa el mecanismoAirPlay deMac |
| Air Mode/Yam Air | Pendiente y fuera del USB actual | La selección normal exige USB; ver dispositivosNetwork enusbmux no demuestra compatibilidad inalámbrica completa |

## Gestos y entrada

**El servidor 0.7 registra contadores de los nombres de entrada conocidos, pero no inyecta todavía touch ni teclado en Windows.** No se guardan coordenadas ni contenido de teclas en esos contadores. Ver `input_event_counts` en el informe de sesión.

| Función de referencia | EstadoWindows/iPad2 | Qué falta comprobar |
|---|---|---|
| Un toque → clic | Pendiente | EventoSingleTap identificado; falta escala/origen, botones y prueba sobre monitor correcto |
| Deslizar uno/dos dedos → scroll | Pendiente | Se identificaScrolled; faltan ejes/unidades/fases |
| Pulsación larga y arrastre/dibujo | Pendiente | Se identificaDragged; faltan inicio/movimiento/fin/cancelación y liberación de botón |
| Doble toque → dobleclic | Pendiente | No asumir queSingleTap2nd significa dobleclic; evidencia lo despacha a otrohandler |
| Dos dedos → menúcontextual | Pendiente | TwoFingerSingleTap identificado; faltan coordenadas y equivalenciaWindows |
| Doble toque con dos dedos → smartzoom | Pendiente | Función y semánticaWindows aún por definir/probar |
| Rotación con dos dedos | Pendiente | Evento/capacidad del receptor y aplicaciónWindows por comprobar |
| Deslizar dos dedos → atrás/adelante | Pendiente | Mapeo de gesto y acciónWindows; evitar enviar acciones a otra ventana |
| Pinch → zoom pantalla completa | Pendiente | Separar zoom local del receptor y transformación de coordenadas del host |
| Pinch → zoom cadaapp | Pendiente | Además de implementaciónWindows, depende de la edición/capacidad del receptor existente |
| Tres dedos → cambiarSpace | Pendiente, equivalente propuesto | EnWindows podría cambiar escritorio virtual; no implementado |
| Tres dedos → teclado onscreen | No prometido en USB actual | La referencia lo atribuye aYamAir; investigar receptor instalado, no asumir paridad |
| Teclado y ratón conectados al iPad | Pendiente enhostWindows | KeyboardEvent/HoverEvent existen, pero payload y aplicación segura sin validar |

El análisis estático conservado confirma campos x/y de algunos taps y despacho de otros eventos. No determina unidades de coordenadas ni todos los handlers. La siguiente prueba debe observar gestos concretos y validar esquinas/centro antes de introducir clics. No se requiere instalar una app nueva para esa investigación.

## Funciones condicionadas por hardware/sistema

| Función | Alcance en este equipo |
|---|---|
| Split View y resolución al redimensionar | El iPad2 no figura entre los modelos que Apple admite para SplitView en iOS9. No es una brecha del hostWindows que podamos resolver aquí |
| Retina/non-retina en SplitView | No es objetivo del panel1024×768 del iPad2 y no permite habilitarSplitView en esehardware |
| Drag&Drop entre apps de iPad y ordenador | La referencia lo exige desdeiOS11; el equipo está eniOS9.3.5. No aplica al objetivo actual |
| Arrastrar múltiples elementos/clipboard entreiOS/macOS | No implementado y fuera del dispositivo/sistemaactual; no se sustituyeCommand/Fn por teclasWindows sin diseñar el flujo |
| Teleprompter | Viable como transformación a investigar, todavía no implementada; requerirá mantener coherentes imagen/cursor/touch |
| ApplePencil/YamPad | Producto/capacidad diferente; el iPad2 no es un dispositivoApplePencil |

Fuente primaria de modelosSplitView: [Apple, AdoptingMultitasking, tabla de dispositivos](https://developer.apple.com/library/archive/documentation/WindowsViews/Conceptual/AdoptingMultitaskingOniPad/index.html). La referencia sobre Drag&Drop/iOS11 y las funcionesmacOS procede del texto aportado porGerman.

## Orden que seguimos

1. Fluidez y recuperación: medir captura/compresión, escritura, colas y presentación; comparar códec sólo con recepción validada. No dar por resuelto lag por mostrar másFPS enviados.
2. Touch básico: tap, arrastre, scroll, contextmenu; validar esquema y transformaciones en el receptoractual. Limpiar botones/teclas al desconectar.
3. Preferencias y uso diario: modos/posición/energía ya añadidos; luego inicio de sesión, reconexión y mejor preparación/GUI.
4. Teleprompter y gestos avanzados después de la entrada básica. No añadirAirPlay/TouchBar/SplitView como casillas sin soporte.

Conservar Yam como receptor durante esta etapa mantiene sus requisitos y las capacidades de su edición. Relevo no modifica su binario ni su licenciamiento. Los avisos de dependencias propias permanecen enTHIRD_PARTY_NOTICES.txt.

## Prueba de fluidez 0.8

Esta entrega prioriza rendimiento: capturaWindowsdirectaDIB/BitBlt, menoscopias y encoder **libjpeg-turbo3.2.0 SIMD** incluido en el ejecutablex64. No necesita instalar FFmpeg ni una DLL manualmente. El formato enviado continúa siendoJPEGcompatibleconlaappactual.

La ventana indica **JPEG SIMD** cuando la ruta nativa está activa. El primer uso realiza una autoprueba pequeña de codificación/decodificación; si falla la carga o la compresión, conserva **JPEG Go** y registra el motivo en last-session.json. El caché temporal se retira al cerrar normalmente la app. No fuerces el cierre durante la prueba.

Para comparar con la medición informada porGerman, conservar **1024×768,30FPS y calidad60**. La versión0.8 no cambia los ajustes guardados. Registrar FPSenviados, capturap95, escriturap95 y si el puntero/scroll se sienten más inmediatos. Si la tasa sube pero el lag visual persiste, revisar recepción/presentación y códec; no declarar resuelto por alcanzar30FPSenviados.

En una prueba aislada deMacarm64, mismaimagen/calidad60, GoJPEG tardó16–20ms y la bibliotecanativa1,9–2,3ms porcuadro. No es una medición de la capturaWindows ni de la latencia del iPad. Estas rutas nuevas requieren prueba real en Windows. El informe añade control_roundtrip_recent_p95_ms como ida/vuelta decontrol, no como latencia de presentación.

Licencias y atribuciónIJG completas en THIRD_PARTY_NOTICES.txt; DLLoficial fijada porhash y cargada desde ruta privada verificada. H264 sigue sin implementarse. Desde0.9 se puede solicitar60FPS.

## Opciones 0.9

FPS solicitados: **10,15,20,30 y60**. CalidadJPEG: **60,70,80,90,95 y100**. Los ajustes anteriores y los valorespredeterminados se conservan; cambia estas opciones con la sesión detenida y vuelve a iniciar.

Para comparar fluidez, prueba primero60FPS manteniendo1024×768 yQ60. Después aumenta calidad a80/90/95/100 de forma independiente. Calidad100 sigue siendoJPEG con pérdidas y no garantiza mayorfluidez; aumenta trabajo y tráfico. 60FPS es objetivo de envío, no garantía de presentación en el iPad. El encoderSIMD y la medición de tasa real se conservan.

La consola acepta `--fps 60 --quality 95`. La implementación de calidadconfigurable se aplica a capturaWindows; no se afirma paridad de captura de otros sistemas. Las opciones nuevas están comprobadas con pruebas de validación/persistencia/negociación y compilaciónWindows; falta probar sus tasas y aspecto en el dispositivo.
