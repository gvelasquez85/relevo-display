> Actualización posterior del usuario, 24 de octubre de 2026: el dispositivo real tiene **iOS 9.3.5**, y ese mínimo es suficiente. Las referencias siguientes a iOS 7 y a soporte obligatorio de iOS 5.1.1 describen el supuesto anterior y quedan sustituidas para la siguiente implementación.

# Relevo Display: instalación del cliente propio en iPads antiguos

Investigación documental: 24 de octubre de 2026. Responsable: Renzo Raines (kipp), por encargo de Hannibal. No se instalaron herramientas, perfiles, certificados ni apps; no se accedió a cuentas Apple ni se compró una membresía.

## Conclusión

Hay un camino concreto **sin pagar membresía** para investigar en el iPad 2 con iOS 7: construir nuestro IPA de 32 bits y probar firma/instalación con cuenta gratuita mediante Sideloadly, cuyo proyecto declara soporte desde iOS 7. Esto es una **capacidad publicada por su proveedor, no una prueba realizada en nuestro iPad**. El pago de Apple Developer ofrece distribución de desarrollo/Ad Hoc y una renovación menos frecuente, pero **no resuelve por sí solo** compatibilidad de compilador, SDK, firma o servicios Apple con iOS antiguo.

Para iOS 5.1.1 no se verificó una ruta moderna gratuita de instalación nativa sin modificar el sistema. Es un objetivo separado pendiente; no extender automáticamente el resultado de iOS 7. TestFlight actual queda descartado para ambos.

## Opciones y evidencia

| Ruta | Coste de membresía | Evidencia documental | iPad 2 / iOS 7 | iOS 5.1.1 |
|---|---|---|---|---|
| Cuenta gratuita / Personal Team | 0 USD | Apple permite pruebas personales; perfiles de 7 días, hasta 3 dispositivos, 3 apps por dispositivo y 10 App IDs; reprovisionar y reinstalar al vencer. [Apple: cuenta](https://developer.apple.com/help/account/basics/about-your-developer-account) | El flujo de Xcode actual no acredita soporte para este sistema. Separar firma de compilación antigua. | No validado. |
| Sideloadly con cuenta gratuita | Sin membresía pagada | El proyecto publica soporte iOS 7+, cuentas gratuitas/pagadas y renovación de firma; sigue dependiendo de Apple y de un IPA compatible. [Proveedor](https://sideloadly.io/), [FAQ](https://sideloadly.io/faq.html) | Candidato principal para una prueba acotada; instalación, apertura y renovación pendientes. | Fuera del rango publicado; no recomendar como solución. |
| Apple Developer: desarrollo / Ad Hoc | 99 USD por año de membresía, o moneda local disponible | Apple permite instalación Ad Hoc en dispositivos registrados, hasta 100 por familia y año. Requiere App ID, certificado y perfil. [Precio](https://developer.apple.com/programs/whats-included/), [dispositivos](https://developer.apple.com/help/account/devices/devices-overview/), [perfil Ad Hoc](https://developer.apple.com/help/account/provisioning-profiles/create-an-ad-hoc-provisioning-profile/) | Candidato; no garantiza aceptación de una firma nueva por iOS 7. | Pendiente de prueba independiente. |
| Sideloadly con cuenta pagada | Membresía Apple anterior | Su FAQ declara firmas de hasta un año. Hay que verificar `ExpirationDate` del perfil emitido: no asumir permanencia ni prometer un año exacto. [FAQ](https://sideloadly.io/faq.html) | Candidato sin alterar iOS; no probado. | Sin soporte publicado. |
| TestFlight | Incluido en membresía Apple | Cliente actual requiere iOS/iPadOS 16+; builds hasta 90 días. [Apple TestFlight](https://testflight.apple.com/) | Excluido. | Excluido. |
| Cliente web local en Safari | Sin membresía ni IPA | Safari antiguo documenta eventos touch/gestos; el vídeo tiene restricciones propias. [Touch](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/HandlingEvents/HandlingEvents.html), [vídeo histórico](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/Using_HTML5_Audio_Video/AudioandVideoTagBasics/AudioandVideoTagBasics.html) | Alternativa experimental por LAN: imágenes JPEG/MJPEG y entrada táctil. Rendimiento no probado. | Alternativa experimental; requiere prueba real. |

La FAQ de Sideloadly declara límites de apps distintos en iOS 7–9 frente a iOS posteriores. Para planificar usamos el límite conservador publicado por Apple; no dependemos de excepciones históricas para que el producto funcione.

## El problema de las herramientas es distinto al precio

Apple distingue explícitamente SDK, deployment target y soporte de instalación/depuración del dispositivo. Su [matriz vigente de Xcode](https://developer.apple.com/xcode/system-requirements/) no ofrece iOS 5/7 como destino actual soportado. Su [guía de arquitecturas](https://help.apple.com/xcode/mac/current/en.lproj/dev34b59f90c.html) distingue `armv7` para destinos anteriores a iOS 11. Las [notas históricas de Xcode 7](https://developer.apple.com/library/archive/documentation/Xcode/Conceptual/RN-Xcode-Archive/Chapters/xc7_release_notes.html) documentan SDKs antiguos, pero no demuestran que esos Xcode puedan autenticarse y aprovisionar hoy.

**Inferencia de ingeniería:** necesitamos una cadena histórica reproducible para compilar el cliente Objective-C/C de 32 bits, y posiblemente separar compilación, firma e instalación. Un Mac Intel con macOS/Xcode compatibles podría ser necesario; no se compró ni configuró ese entorno. Cambiar `MinimumOSVersion` de un IPA moderno no elimina APIs ausentes ni agrega instrucciones ARM de 32 bits.

Cuatro pruebas distintas deben pasar: (1) generar el binario armv7 compatible, (2) obtener firma/perfil que el dispositivo acepte, (3) instalar y arrancar, (4) depurar o recopilar registros. Una instalación exitosa no garantiza depuración interactiva desde Xcode moderno.

## Alternativa web sin firma

Un cliente servido por el equipo Windows/Linux podría mostrar imágenes y enviar touch por red local. Es una **hipótesis de respaldo**, no sustituto acreditado del cliente nativo de baja latencia. No asumir WebRTC, WebCodecs, service workers o capacidades PWA modernas en Safari de iOS 5/7. Primero probar `<img>` JPEG y eventos touch con JavaScript compatible; después MJPEG si el navegador real lo acepta y no acumula memoria.

Safari no obtiene por sí solo acceso al túnel usbmux que hoy usa una app nativa. Un navegador por Wi-Fi no equivale a tener vídeo por USB: para conservar nuestro transporte USB normalmente hace falta un servicio nativo en el iPad o una ruta IP por USB separada, que aquí no se validó. Tampoco debe exponerse un control remoto anónimo a toda la LAN; el prototipo necesitaría emparejamiento y sesión acotada.

## Rutas que no adoptamos como base

- AltStore PAL requiere un sistema moderno y región admitida; no resuelve dispositivos iOS 5/7. [Requisitos publicados](https://altstore.io/download).
- Enterprise es distribución interna para organizaciones elegibles, no una solución genérica para todos nuestros clientes. [Apple: uso Enterprise](https://support.apple.com/en-kz/guide/deployment/depce7cefc4d/web).
- Jailbreak podría abrir una vía de instalación en hardware antiguo, pero cambia el sistema y su seguridad. Se mantiene como investigación opcional separada; no se ejecutó, no es requisito propuesto y no es una instalación de consumo sin mantenimiento.
- No usar certificados empresariales ajenos, cuentas prestadas, evasión de revocación ni alteraciones de fecha como estrategia de producto. No confundir no pagar membresía con quitar el licenciamiento de software ajeno.

## Experimento recomendado antes de construir toda la app

1. Mantener instalada la app de monitor que ya funciona. Confirmar versión exacta de iOS y arquitectura del equipo de referencia sin actualizarlo.
2. Crear una app propia mínima, con identificador separado, que muestre versión y reciba toques localmente. Debe compilar en armv7 y no incorporar binarios ni recursos de terceros propietarios.
3. Preparar IPA y registrar herramienta/SDK, arquitecturas, deployment target y hashes. No incluir UDID ni credenciales en informes públicos.
4. Primera prueba: cuenta gratuita y Sideloadly oficial. La autenticación pertenece al dueño de la cuenta; nuestro programa no debe recopilar contraseñas Apple. Verificar el perfil real, instalar, abrir, cerrar, reiniciar y abrir de nuevo; medir duración del proceso y registrar error exacto si falla.
5. Comprobar renovación al vencer el perfil y preservación de ajustes. Si firma o instalación falla, clasificar por servicio Apple, certificado/perfil, arquitectura, APIs o instalador; comprar membresía sólo si resuelve el bloqueo identificado.
6. Si se dispone de membresía pagada, repetir con desarrollo/Ad Hoc y comprobar caducidad real del perfil. No prometer instalación indefinida.
7. Sólo después añadir transporte USB, un fotograma y touch de retorno. iOS 5.1.1 requiere su propio hardware y ensayo de aceptación.

**Criterio de continuar:** app propia instala y arranca repetidamente en iPad 2/iOS 7, firma/renovación entendidas y procedimiento reproducible sin tocar la app existente. **Criterio de replantear:** no se obtiene firma aceptada o el mantenimiento resulta incompatible con uso diario; evaluar cliente web y limitar promesas hasta contar con prueba física.

Estado: investigación documental completada; ninguna de estas rutas nuevas está validada en el dispositivo del usuario. La membresía habilita distribución Apple; no elimina licencias obligatorias de dependencias ni reemplaza el trabajo técnico de compatibilidad.
