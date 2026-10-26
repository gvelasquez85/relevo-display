# Arquitectura y desarrollo

Estado de referencia: 26 de octubre de 2026. Relevo tiene dos rutas de recepción en el iPad: una compatible con la app instalada y otra con un cliente propio. Comparten módulos del host, pero sus protocolos y el estado de validación son diferentes.

## Componentes

| Ruta | Responsabilidad |
| --- | --- |
| `cmd/relevo` | Aplicación principal; GUI Windows, configuración y diagnóstico de la entrega unificada. |
| `internal/nativeapp` | Sesiones y panel de pruebas del cliente propio, accesible desde el GUI principal. |
| `cmd/relevo-native`, `cmd/relevo-lab` | Entradas auxiliares de desarrollo; no constituyen entregas adicionales para el usuario. |
| `internal/wintray`, `internal/appstate` | Ventanas, icono de notificación, controles, preferencias y resultados. |
| `internal/desktop` | Captura de escritorio y adaptación a las APIs de cada sistema. |
| `internal/winvirtual`, `internal/windriver` | Gestión de pantalla virtual Windows, detección y restauración de recursos. |
| `internal/usbmux` | Enumeración y conexión mediante los servicios locales de Apple. |
| `internal/wire`, `internal/session` | Transporte y sesión del receptor compatible. |
| `internal/nativewire`, `internal/nativesession` | Protocolo RVL1 y sesión del cliente propio. |
| `internal/jpegfast` | Codificación JPEG, con aceleración nativa donde está disponible y alternativa Go. |
| `internal/wininput`, `internal/winfocus`, `internal/remoteinput` | Entrada remota, contactos, teclado y detección limitada de campos editables. |
| `internal/winaudio`, `internal/winplayback` | Audio de Windows hacia el iPad y escucha local del micrófono recibido. |
| `internal/mediabridge`, `drivers/windows-media` | Formatos, colas y componentes para futuros dispositivos multimedia virtuales. |
| `clients/ios` | Aplicación UIKit/Objective-C del iPad, proyecto y compilación ARMv7. |

## Pantalla y ciclo de sesión

El host obtiene una imagen de escritorio o un patrón, codifica JPEG y lo transmite por USB. La pantalla extendida de Windows utiliza un proveedor de pantalla virtual; no depende de que el iPad anuncie una pantalla física USB. El cierre y la desconexión deben liberar entradas, detener medios y retirar los recursos de pantalla pertenecientes a la sesión.

La GUI conserva resolución, FPS solicitados y calidad. Los FPS enviados, tiempos de captura y escritura son mediciones del host; no prueban la latencia de presentación del iPad. JPEG sigue siendo el formato de vídeo implementado. La evaluación de H.264 no implica soporte activo.

## Cliente propio y RVL1

El iPad escucha en loopback, puerto 52742; el host accede mediante USBMux. RVL1 negocia capacidades antes de habilitar pantalla, cámara, audio, multitouch, teclado o modos de entrada. Los mensajes tienen tamaños acotados y la sesión incorpora confirmaciones y vigilancia de desconexión. La confirmación de imagen corresponde a su asignación en la vista iOS, no a una medición física del refresco.

El cliente ofrece modos pantalla, teclado y mouse. El teclado automático depende de metadatos de campos nativos Edit/RichEdit de Windows; los campos personalizados no tienen cobertura completa. El modo dedicado permite activación manual. La cámara y el micrófono requieren permisos y activación explícita, y se detienen al salir o desconectar. En esta revisión, captura de micrófono y reproducción USB se prueban por separado.

El cliente ARMv7 se compiló para iOS 9.3.5 con el SDK oficial 15.2, se firmó e instaló. El fallo de verificación posterior de Apple impide validar su ejecución real. La ruta compatible continúa siendo la opción física probada para pantalla.

## Plataformas

- **Windows x64:** GUI principal, captura, pantalla virtual, cursor, configuración y panel de nuevas funciones. Las ampliaciones del cliente propio requieren pruebas físicas una vez desbloqueada su ejecución.
- **Linux amd64/arm64:** Hyprland dispone de una ruta con `hyprctl` y `wf-recorder`. GNOME/KDE utilizan portal ScreenCast/PipeWire y GStreamer; extensión depende de que el compositor anuncie una fuente virtual. El código no acredita funcionamiento en Ubuntu, Omarchy, Bazzite o Raspberry Pi sin pruebas en esos equipos.
- **macOS:** captura mediante `screencapture`, sujeta a los permisos del sistema, y patrón de prueba. No existe una implementación propia de extensión de pantalla en esta ruta.
- **Wi-Fi:** enumerar un dispositivo de red no establece una sesión. La propuesta exige emparejamiento y transporte autenticado; no se ha habilitado un listener LAN como sustituto del USB.

## Dispositivos multimedia virtuales

La cámara requiere una fuente COM/Media Foundation registrada, alimentación de imágenes y validación de activación en Windows. Existe código derivado de una muestra de Microsoft, adaptación de JPEG y comportamiento negro ante datos ausentes. Falta completar y validar el transporte local, instalación y enumeración real.

El micrófono requiere un endpoint de captura de Windows y su controlador firmado, además del alimentador PCM. Existen conversión a 48 kHz, colas acotadas, validación de paquetes y silencio ante desconexión. Esos componentes no registran por sí solos un micrófono. La escucha local con WinMM/WASAPI y la vista previa de cámara no sustituyen estos dispositivos.

Consulta las limitaciones detalladas en [`drivers/windows-media`](../drivers/windows-media/README.md). No hay dependencia de OBS prevista en la entrega.

## Compilación y comprobación de desarrollo

El host usa Go 1.23 o posterior; las dependencias están fijadas en `go.mod` y `go.sum`. El script `scripts/build.sh` ejecuta pruebas con detector de carreras, análisis estático y compilación Windows x64, y genera la entrega unificada con `scripts/package.py`. Las dependencias de compilación no son instrucciones de instalación para el usuario.

El cliente iOS dispone de `clients/ios/build-with-sdk.sh` y un proyecto Xcode. Se necesita un SDK oficial con soporte ARMv7 y firma/provisión válidas para el dispositivo; la presencia de una herramienta de firma no garantiza superar la verificación de iOS.

Los componentes C/C++ de medios se construyen mediante CMake. Sus pruebas portables verifican formatos, límites y colas; la parte de Windows requiere las herramientas y SDK/WDK indicados en su propio README. Compilar Go para Windows no verifica COM ni un controlador de audio.

Antes de integrar cambios, ejecutar las comprobaciones pertinentes y registrar qué se probó realmente. Mantener los resultados del usuario dentro del GUI y el diagnóstico exportable. No incorporar al repositorio herramientas descargadas, SDKs, perfiles, certificados privados, credenciales, capturas personales ni artefactos de extracción de Xcode.

## Comprobaciones con el receptor compatible (0.12)

El panel `Pruebas con Yam` reutiliza el mismo runner y captura de la pantalla habitual. Ajusta resolución, modo y posición en la ventana principal; calidad y FPS se seleccionan en el panel. Las pruebas de 30/60 duran 20 segundos y sólo comparan configuraciones equivalentes. Desconexión, UAC y observación de entrada duran hasta 45 segundos. Detener no se marca como prueba completada.

El diagnóstico de desconexión distingue eventos de bloqueo/segundo plano/cierre, pérdida de transporte, timeout de pong y fallo de formato. Sólo los primeros casos, tras vídeo real y limpieza sin error, se reportan como desconexión observada. Volver a conectar y empezar otra prueba comprueba una nueva sesión; no se promete reconexión automática.

`desktop.CaptureStatus` cuenta pausas por inaccesibilidad/cambio del escritorio de entrada y su recuperación. No identifica por sí solo la causa UAC. La acción GUI solicita elevación del ejecutable con `--uac-probe`; ese modo termina antes de USB, captura o instalación. La aplicación no entra en el escritorio seguro. Las comprobaciones visuales de imagen, posición y reorganización de ventanas siguen siendo humanas.

`yamchecks` resume métricas y esquemas de entrada con nombres/tipos aprobados. No guarda valores de teclas, texto o coordenadas. No habilita control de Windows mediante eventos Yam. La sesión interrumpe escrituras pendientes al cancelar y espera la salida del lector antes de retornar; la captura retira y verifica su pantalla antes de informar limpieza.
