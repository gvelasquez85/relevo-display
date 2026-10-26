# Relevo Display

Reutiliza iPads antiguos como pantallas de computador. El equipo de referencia es un **iPad 2 con iOS 9.3.5** y Windows 11. Este repositorio reúne el host de escritorio, el cliente iOS propio y los prototipos de Linux y macOS.

## Estado del proyecto

| Componente | Estado actual |
| --- | --- |
| Windows, pantalla compatible | Funciona con la app Yam Display ya instalada en el iPad. El usuario confirmó imagen y mejor fluidez, con unos 45–50 FPS enviados; no es una medición de latencia ni de cuadros presentados. |
| Windows, entrega actual | **0.12.2 experimental**, una aplicación con pantalla, «Pruebas y nuevas funciones» y «Exportar diagnóstico». |
| Cliente iOS propio | UIKit/Objective-C, ARMv7, mínimo iOS 9.3.5. Compilado, firmado e instalado, pero el iPad rechaza la verificación de Apple y aún no permite probar su ejecución. |
| Touch, teclado, cámara y audio propios | Código y pruebas automatizadas disponibles. Las pruebas con el cliente propio en el dispositivo siguen bloqueadas por su verificación. |
| Cámara y micrófono virtuales de Windows | Componentes en desarrollo. La vista previa de cámara y la escucha local del micrófono **no crean dispositivos seleccionables** por otras aplicaciones. |
| Linux / Raspberry Pi | Prototipos de captura y extensión según compositor; pendientes de validación física. |
| macOS | Prototipo de captura y patrón de prueba; sin implementación propia de pantalla extendida. |
| Wi-Fi | Diseño y selección de metadatos de transporte; sin sesión inalámbrica implementada. |

La ruta compatible de pantalla se conserva mientras se completa el cliente propio. Relevo no incluye la app propietaria del iPad ni elimina o modifica su licenciamiento.

## Una sola entrega y pruebas desde la aplicación

La entrega vigente se genera como `dist/Relevo-Windows.exe` y su ZIP correspondiente, `dist/Relevo-Windows.zip`. Se mantiene **una sola versión actual**; los artefactos anteriores se archivan. Los binarios de desarrollo y la documentación técnica no son entregas alternativas para el usuario.

Las pruebas, sus resultados y la exportación de diagnóstico se realizan desde el GUI de Windows. «Pruebas con Yam» ofrece mediciones de 30/60 FPS, comprobación de pantalla, desconexión, UAC y observación de eventos de entrada. La comparación exige iguales resolución, calidad, modo y posición; los contadores son del host. La prueba UAC incluye una solicitud de elevación de un proceso propio sin operaciones de instalación. Los gestos se observan sin inyectar entrada ni guardar texto. No se requieren comandos ni archivos BAT para ese flujo. El transporte USB sigue necesitando los servicios de Apple en Windows; no se redistribuyen como parte de este repositorio. El instalador completo y la firma de todos los componentes siguen pendientes.

## Código y documentación

- [`cmd/relevo`](cmd/relevo): entrada de la aplicación de escritorio y entrega unificada de Windows.
- [`clients/ios`](clients/ios): cliente iPad propio, proyecto Xcode y fuentes Objective-C.
- [`internal`](internal): protocolos, sesiones, captura por plataforma, GUI, entrada y audio.
- [`drivers/windows-media`](drivers/windows-media): componentes de cámara y micrófono virtuales en desarrollo.
- [`scripts`](scripts): compilación y empaquetado para desarrollo.
- [`docs`](docs/README.md): arquitectura, decisiones, límites y documentación técnica.

Consulta la [arquitectura y requisitos de desarrollo](docs/architecture.md) antes de modificar o compilar. Los documentos antiguos conservan el contexto de cada revisión; el estado vigente se resume aquí y en el índice de documentación.

## Validación y licencias

Las pruebas automatizadas, los dispositivos simulados y la compilación cruzada ayudan a verificar el código, pero no acreditan funcionamiento físico en Windows, Linux o el iPad. Cada ampliación debe distinguir esas comprobaciones de las pruebas reales del usuario.

Las dependencias y sus atribuciones se documentan en [`THIRD_PARTY_NOTICES.txt`](THIRD_PARTY_NOTICES.txt). El código de terceros conserva sus licencias y procedencia. La disponibilidad del repositorio no concede por sí sola una licencia de distribución del código propio.
