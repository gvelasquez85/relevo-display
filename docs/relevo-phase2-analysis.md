> Actualización posterior del usuario, 24 de octubre de 2026: el dispositivo real tiene **iOS 9.3.5**, y ese mínimo es suficiente. Las referencias siguientes a iOS 7 y a soporte obligatorio de iOS 5.1.1 describen el supuesto anterior y quedan sustituidas para la siguiente implementación.

# Relevo Display: transición al cliente propio

24 de octubre de2026. Consolidación Hannibal de tres análisis ejecutados: Murdock funciones/usos, RenzoRaines instalación, worker requisitos del cliente. No se instalaron apps, perfiles ni herramientas, no se compraron membresías y no se modificó el iPad.

## Decisiones y estado

Equipo real de referencia: iPad2/iOS7/Windows11. German confirma mejora importante y funcionamiento de pantalla reconocida por Windows, con lag aún visible. La fase1 sigue condicionada a mejorar fluidez y obtener aceptación de German. Cambiar el códec es una hipótesis: captura/conversión/codificación/transporte/decodificación/presentación pueden añadir latencia. El emisor actual sólo mide sus etapas y FPS enviados; todavía no mide presentación en iPad. Los umbrales25FPSvisibles/p95≤150ms de documentos previos son propuestas, no acuerdos ni resultados.

## Resumen de los tres frentes

1. Funciones/usos: priorizar fluidez, recuperación y touch; después perfiles/preparación simple. RaspberryPi como monitorprincipal sin pantalla física tiene prueba propia de arranque/entrada; Linux se valida por compositor. Audio/webcam después del clientepropio. AirPods/iOS7, Bluetooth general y control arbitrario de apps siguen sin demostrar.
2. Clientepropio: Objective-C/UIKit32bits y protocoloRelevo independiente, imagen1024x768, touchdown/move/up/cancel, reconexión, límites de memoria y colas, señalización de vídeo y diagnóstico. JPEG inicial; H264 se valida aparte, porque las APIs públicas habituales de decodificación que investigamos requiereniOS8. SoporteiOS5.1.1 es un hito separado, no resultado automático de probariOS7.
3. Instalación: primer candidato gratuito es un IPAarmv7 propio firmado con cuentaApplegratuita mediante Sideloadly, cuyo proveedor declara iOS7+. La firma gratuita caduca a7días y requiere renovación. Ruta pagada: AppleDeveloper99USD/año, desarrollo/AdHoc para dispositivosregistrados y vigenciareal del perfil. TestFlight16+ no sirveparaesteiPad. Ninguna ruta se probó en el dispositivo, y pagar no resuelve compilación/SDK. Para iOS5.1.1 no encontramos una ruta gratuita moderna nativa validada; explorar por separado y conservar webLANexperimental como respaldo.

Referencias de instalación: [ApplecuentaPersonalTeam](https://developer.apple.com/help/account/basics/about-your-developer-account), [Appleprecio](https://developer.apple.com/help/account/membership/program-enrollment), [AppleAdHoc](https://developer.apple.com/help/account/provisioning-profiles/create-an-ad-hoc-provisioning-profile/), [Sideloadlyoficial](https://sideloadly.io/), [Xcodeversiones](https://developer.apple.com/xcode/system-requirements/). Consultadas24octubre2026.

## Independencia y licenciamiento

No se encontró un comprobador de licencia del productooriginal en cmd/internal durante la inspección acotada. La dependencia actual está en el receptor instalado en iPad: reemplazarlo por nuestra app evita que nuestro funcionamiento dependa de ese receptor. No se elimina ni modifica su sistema de licencia; no se ha construido aún su reemplazo. Se mantienen evidencias históricas e identificadores de compatibilidad hasta abandonar ese adaptador.

Go, plist, godbus y MTTVDD siguen siendo componentes de terceros. Sus avisos no se borran. Se corrigió únicamente el encabezado obsoleto "Yam Trial0.2" en THIRD_PARTY_NOTICES.txt para identificar RelevoDisplay y el proveedorMTTVDD; los textos de licencia quedaron intactos. La corrección está en fuente, no se publicó un paquete nuevo en este encargo.

La membresía de desarrolloApple es distinta de cualquier licencia del receptor y de las licencias de los componentes. No se tomó una decisión sobre licencia comercial/open-source del código propio en este análisis.

## Entorno comprobado y primer ensayo

En este Mac, xcode-select apunta a /Library/Developer/CommandLineTools. No figura Xcode.app en /Applications y xcrun --sdk iphoneos --show-sdk-path falla: no haySDKiOS disponible por la instalaciónseleccionada. LosCommandLineTools no bastan para generar el clienteUIKit. Hace falta resolver toolchainhistórico armv7, posiblemente en entornoIntel/macOScompatible; esto no está probado.

El diagnóstico local Relevo --doctor encontró sólo transportesNetwork y ningúnUSB en esteencargo. No se abrió una sesiónYam ni se consultaron credenciales/seriales. La información previa del usuario de tener el iPad conectado no demuestra disponibilidadUSB actual.

Primer ensayo recomendado: compilar una app propia mínimaarmv7 con identidad separada que muestre versión y responda a toques localmente; firmarla con cuenta gratuita, instalar y abrir sin tocar la app quefunciona. Verificar reinicio/renovación, después imagenUSB y touchretorno. No introducir contraseñasApple en logs ni en nuestro código. Comprar membresía sólo si resuelve una limitación identificada o se necesita la distribuciónpagada.

## Documentos detallados

- [Funciones y usos](relevo-features-and-uses.md)
- [Requisitos clientepropio](relevo-native-client-requirements.md)
- [Instalación pagada/gratuita](relevo-installation-options.md)
- [Evaluación del códec](codec-evaluation-2026-10-24.md)

El resultado de esteencargo es análisis y procedimiento de prueba. No acredita compatibilidadnativa, firma, instalación, H264 en iPad ni nuevasfunciones implementadas.
