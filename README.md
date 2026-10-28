# Relevo Display

Reutiliza iPads antiguos como pantallas de computador. El equipo de referencia es un **iPad 2 con iOS 9.3.5** y Windows 11. Este repositorio reúne el host de escritorio, el cliente iOS propio y los prototipos de Linux y macOS.

## Estado del proyecto

| Componente | Estado actual |
| --- | --- |
| Windows, pantalla compatible | Funciona con la app Yam Display ya instalada en el iPad. El usuario confirmó imagen y mejor fluidez, con unos 45–50 FPS enviados; no es una medición de latencia ni de cuadros presentados. |
| Windows, entrega actual | **0.16.2 experimental**, un instalador/GUI único con instalación, actualización, reparación, desinstalación, pruebas y diagnóstico. El usuario confirmó imagen del acceso con 0.16.0; la transferencia tras introducir la contraseña falló. 0.16.1 inicia el agente desde el servicio y tolera la preparación del escritorio; esa corrección requiere validación física. |
| Cliente iOS propio | UIKit/Objective-C, ARMv7, mínimo iOS 9.3.5. La versión 0.15.0 está compilada y empaquetada sin firma. Una versión anterior fue firmada e instalada, pero el iPad rechazó la verificación de Apple; la ejecución del cliente propio sigue pendiente de validación. |
| Touch y teclado propios | Multitouch hasta 10 contactos, teclado Unicode/atajos, apertura tras un toque en un campo nativo editable, modo teclado y trackpad manuales. Compilado ARMv7; prueba física pendiente por verificación Apple. |
| Cámara y audio propios | Código y pruebas automatizadas disponibles; ejecución física pendiente con el cliente propio. |
| Cámara y micrófono virtuales de Windows | Componentes en desarrollo. La vista previa de cámara y la escucha local del micrófono **no crean dispositivos seleccionables** por otras aplicaciones. |
| Linux / Raspberry Pi | Prototipos de captura y extensión según compositor; pendientes de validación física. |
| macOS | Prototipo de captura y patrón de prueba; sin implementación propia de pantalla extendida. |
| Wi-Fi | Implementado para el cliente propio: QR temporal, TLS 1.2, pin SHA256 y token; interoperabilidad local macOS comprobada. Pendiente validación física en iOS 9 / Windows. |

El GUI permite seleccionar «Yam (actual)» o «Relevo (app propia)» y, para Relevo, USB o Wi-Fi. Wi-Fi requiere elegir una IP privada de Windows y escanear el QR desde el cliente iPad; no necesita los servicios USB de Apple. El cliente propio utiliza 1024×768. La apertura automática del teclado detecta controles Edit/RichEdit nativos; controles personalizados requieren el modo manual. La ruta Windows añade detección UIAutomation de controles Edit accesibles en navegadores. La ruta compatible de pantalla se conserva mientras se valida físicamente el cliente propio. Relevo no incluye la app propietaria del iPad ni elimina o modifica su licenciamiento.

## Una sola entrega y pruebas desde la aplicación

La entrega vigente se genera como `dist/Relevo-Windows.exe` y su ZIP correspondiente, `dist/Relevo-Windows.zip`. Se mantiene **una sola versión actual**; los artefactos anteriores se archivan. Los binarios de desarrollo y la documentación técnica no son entregas alternativas para el usuario.

Las pruebas, sus resultados y la exportación de diagnóstico se realizan desde el GUI de Windows. «Pruebas con Yam» ofrece mediciones de 30/60 FPS, comprobación de pantalla, desconexión, UAC y observación de eventos de entrada. La comparación exige iguales resolución, calidad, modo y posición; los contadores son del host. La prueba UAC incluye una solicitud de elevación de un proceso propio sin operaciones de instalación. Las opciones «Touch/gestos» y «Teclado iPad» habilitan la traducción de eventos Yam a entrada de Windows: toque y arrastre mediante touch de un contacto, clic secundario, desplazamiento, zoom por Ctrl+rueda y teclado HID/Unicode. Los gestos agregados de Yam no exponen los contactos necesarios para multitouch completo. No se guarda texto. La apertura automática del teclado de Yam no está implementada; se recibe el teclado que ofrece el cliente. UAC y aplicaciones elevadas pueden rechazar entrada; la sesión de vídeo continúa. La interacción física de esta integración sigue pendiente de validación en Windows. No se requieren comandos ni archivos BAT para ese flujo. El transporte USB sigue necesitando los servicios de Apple en Windows; no se redistribuyen como parte de este repositorio. El instalador nativo y su servicio están incluidos; la firma de distribución y la validación física de instalación siguen pendientes.

## Instalación administrada de Windows

El mismo EXE descargado abre el instalador; instalado en Program Files abre el GUI
de escritorio. El instalador ofrece instalar/actualizar, reparar, desinstalar,
abrir Relevo y comprobar servicio/dependencias. Registra el servicio LocalSystem y
el inicio del agente de usuario. Conserva los ajustes y los controladores
compartidos al desinstalar. La actualización solicita el cierre normal del GUI;
si retirar la única salida dejaría al usuario sin pantalla, la operación se
rechaza conservando la instalación activa.

El servicio prepara el adaptador sin exigir otra pantalla y supervisa un proceso
limitado a WinSta0\Winlogon en la consola, sólo antes de que exista una sesión de
usuario. Verifica el propietario del transporte USB local de Apple, enlaza los
hilos de captura/entrada al escritorio de acceso y los retira al iniciar sesión.
El usuario confirmó imagen de acceso mediante GDI en Windows con 0.16.0. La
continuidad después de introducir la contraseña falló: 0.16.1 inicia el agente
con el token filtrado y el entorno del usuario, una vez cargado su perfil; también
reintenta la preparación del escritorio y no depende del icono de Explorer. Esa
transferencia sigue pendiente de comprobación física. Esta primera ruta no cubre desbloquear una cuenta ya iniciada ni envía
Ctrl+Alt+Del o cambia sus políticas. Las credenciales no se guardan ni aparecen en
los diagnósticos. El GUI muestra estado del servicio y el resultado de transmisión
previa al acceso. El transporte USB reutiliza el componente Apple ya instalado;
no se incluye ni se instala iTunes silenciosamente.

Al pasar Yam a segundo plano, bloquear el iPad o perder el transporte, el GUI
normal queda esperando su regreso. 0.16.2 vuelve a negociar la sesión por USB
sin exigir «Iniciar», incluso si la conexión original se inició manualmente.
Durante esa espera se conserva el adaptador preparado; la extensión ordinaria
se retira del escritorio, pero una salida principal/única conserva su topología.
«Detener» termina los reintentos y realiza la retirada completa cuando es seguro.
Los fallos de captura, decodificación y protocolo malformado siguen siendo errores
visibles; no se ocultan dentro de reintentos de conexión. La recuperación real
al volver a Yam necesita validación física de esta versión.

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
