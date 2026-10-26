# Cliente propio para iPad 2 / iOS 9.3.5: compilación e instalación

Fecha de comprobación: 24 de octubre de 2026. Responsable: Renzo Raines (`kipp`). Encargo: `relevo-ios935-20261024`. Alcance: herramientas locales, fuentes primarias y pruebas mínimas sin instalar software ni modificar el iPad.

## Actualización comprobada — 25 de octubre de 2026

Xcode13.2.1 oficial ya se descargó. Se verificó firmaApple y SHA1 completo del Content, y se extrajo SDK15.2 con 6.862entradas/147.019.245bytes, resolviendo hardlinkscompartidos sin extraerXcodeentero. Compilación real con Appleclang21 en esteMac correcta: ejecutable MachO armv7, LC_VERSION_MIN_IPHONEOS9.3.5, SDK15.2, LC_DYLD_INFO_ONLY compatible con el modelo antiguo de carga. Se generó `dist/Relevo-iPad-0.4-unsigned.ipa`.

No se ha firmado ni instalado. Cero identidades de firma/perfiles locales detectados; portalApple muestra invitación a inscribirse sin recursos de certificados en la sesión consultada. iPad detectado porNetwork, noUSB. SideloadlyMac descargado del sitiooficial se inspeccionó sin ejecutarlo: codesign informa ausencia de firma, Gatekeeperlo rechaza; no se añadió excepción ni se instaló. El procedimiento de firma/instalación requiere resolver esas condiciones y participación del titular de cuenta.

Los apartados históricos siguientes describen el estado previo al SDK y quedan actualizados por esta comprobación.

## Resultado

El objetivo aceptado por German pasa a iOS 9.3.5. El iPad 2 requiere código **armv7 de 32 bits**: instalar el Xcode actual por sí solo no resuelve ese destino. Existe una ruta histórica oficial mediante Xcode 13.2.1; falta obtener su SDK y comprobar el binario resultante, firma e instalación en este equipo.

**Bloqueo actual concreto:** sólo están instaladas Command Line Tools; `xcrun --sdk iphoneos --show-sdk-path` falla porque falta el SDK de iPhoneOS. No existe todavía una app iOS compilada y firmada. La prueba local demuestra que el compilador y enlazador actuales aún procesan armv7; no demuestra compatibilidad completa con UIKit ni ejecución en iOS.

## Evidencia local reproducible

Herramientas consultadas únicamente en `/Library/Developer` y `.tools` del proyecto:

```text
xcode-select -p
/Library/Developer/CommandLineTools

xcrun clang --version
Apple clang version 21.0.0 (clang-2100.1.1.101)
Target: arm64-apple-darwin25.5.0

xcrun --sdk iphoneos --show-sdk-path
xcrun: error: SDK "iphoneos" cannot be located
```

`/Library/Developer/CommandLineTools/SDKs` contiene SDK macOS (15 y 26), ningún SDK iPhoneOS. Los archivos de prueba quedan bajo `.tools/ios935-probe/` (directorio ignorado por Git).

```sh
printf 'int main(void) { return 0; }\n' > .tools/ios935-probe/probe.c
xcrun clang -target armv7-apple-ios9.0 -c .tools/ios935-probe/probe.c \
  -o .tools/ios935-probe/probe.o
file .tools/ios935-probe/probe.o
# Mach-O object arm_v7
```

El compilador avisa que está usando sysroot macOS para destino iPhone: la prueba no incluye headers de plataforma y no sirve como compilación de la app.

```sh
xcrun ld -arch armv7 -platform_version ios 9.0 9.0 -e _main \
  -o .tools/ios935-probe/probe .tools/ios935-probe/probe.o
# Falla: dynamic executables or dylibs must link with libSystem.dylib for architecture armv7

xcrun ld -arch armv7 -static -e _main \
  -o .tools/ios935-probe/probe-static .tools/ios935-probe/probe.o
file .tools/ios935-probe/probe-static
# Mach-O executable arm_v7
```

El ejecutable estático es sólo una sonda del enlazador: no se ejecutó, no se firmó y no es una app iOS instalable.

## Ruta recomendada y alternativa

1. Obtener **Xcode 13.2.1 de Apple**, con sesión de Apple Account. Apple indica que descargar versiones antiguas no requiere membresía de pago. No se accedió a ninguna cuenta ni se descargó el archivo.
2. Verificar que el paquete contiene SDK iPhoneOS 15.2, stubs armv7 de libSystem/UIKit/Foundation y herramientas de enlace. No hace falta específicamente un SDK iOS 9 si el SDK posterior conserva armv7 y se fija deployment target 9.0; debe verificarse el conjunto real antes de prometerlo.
3. Probar primero el compilador actual con `-target armv7-apple-ios9.0 -isysroot <SDK oficial extraído>` y enlace contra los frameworks antiguos. Usar Objective-C/UIKit y APIs disponibles en iOS 9 evita depender de runtime Swift moderno. Esta combinación en macOS actual es una **hipótesis de viabilidad**, no una configuración que Apple documente como soportada.
4. Si el SDK o herramientas históricas no funcionan en este Mac, usar un Mac/entorno compatible con Xcode 13.2.1 (por ejemplo Big Sur/Monterey en hardware que pueda ejecutarlos). No se afirma que el Xcode 13 completo funcione en macOS 26 ni que bajar macOS en este Apple Silicon concreto sea posible. Un Mac Intel antiguo es una alternativa, no un requisito demostrado.
5. Crear una app mínima con bundle ID propio; comprobar `file`/`lipo` armv7 y `LC_VERSION_MIN_IPHONEOS` 9.0, firmar y probar apertura real. Después validar un JPEG y touch; luego H.264 y medidas de latencia. No reemplazar Yam durante esta prueba.

Apple documenta que Xcode 13.2.1 incluye SDK iOS 15.2 y depura dispositivos iOS 9 o posteriores. Las notas de Xcode 13.2 también mencionan correcciones para armv7. Apple eliminó en Xcode 14 la compilación armv7 y destinos anteriores a iOS 11. Estas fuentes justifican seleccionar la rama 13 y no la actual; no garantizan que sus herramientas históricas funcionen sin cambios en macOS 26.

## Firma gratuita y pagada

Sideloadly declara soporte iOS 7 o posterior, Apple Silicon, firma gratuita con validez de siete días y de pago hasta un año. Es documentación del proveedor, **no una prueba realizada en este iPad ni una garantía de que el servicio de firma de Apple acepte hoy la cuenta/configuración**. El usuario introduciría sus credenciales en la herramienta elegida; no deben guardarse en scripts, informes ni el repositorio. No se instaló Sideloadly.

La primera prueba propuesta usa una app propia junto a Yam, una cuenta gratuita y una instalación real. Pagar después puede facilitar duración/distribución, pero no agrega armv7 al SDK ni elimina la necesidad de probar.

## ¿Modificar directamente Yam instalada desde el Mac?

En un iPad con iOS estándar no es un flujo de edición directa: iOS valida firmas de código y detecta alteraciones. El acceso USB normal no concede escritura sobre el ejecutable de otra app ni una sesión de depuración de su binario App Store. No se comprobó jailbreak y no se propone cambiar la seguridad del iPad. La ruta práctica es **cliente propio, firmado, con identidad distinta**, conservando la app funcional. No se modificó ni extrajo el ejecutable instalado.

## Criterio de aceptación del siguiente experimento

App propia armv7, mínimo iOS 9.0, instalada y abierta en el iPad 2 con iOS 9.3.5; muestra un patrón recibido y registra coordenadas táctiles sin alterar Yam. Firma, red/USB, decodificación de vídeo y estabilidad deben validarse por separado. Estado actual: pendiente de SDK, firma e instalación.

## Fuentes primarias consultadas

- [Apple: Xcode 13.2.1 Release Notes](https://developer.apple.com/documentation/xcode-release-notes/xcode-13_2_1-release-notes): SDK 15.2, on-device debugging iOS 9+, macOS 11.3 mínimo.
- [Apple: Xcode 13.2 Release Notes](https://developer.apple.com/documentation/xcode-release-notes/xcode-13_2-release-notes): referencias explícitas a compilación armv7.
- [Apple: Xcode 14 Release Notes](https://developer.apple.com/documentation/xcode-release-notes/xcode-14-release-notes): elimina armv7/armv7s y deployment anterior a iOS 11.
- [Apple: 32-bit app support discontinued in Xcode 14](https://developer.apple.com/news/upcoming-requirements/?id=06062022a), aviso de 6 de junio de 2022.
- [Apple: Xcode Resources](https://developer.apple.com/xcode/resources/): versiones antiguas mediante Apple Account, sin membresía requerida. [Catálogo oficial](https://developer.apple.com/download/all/?q=Xcode%2013.2.1); acceso autenticado no realizado.
- [Sideloadly: FAQ](https://sideloadly.io/faq.html): compatibilidad declarada y duración de firma.
- [Apple Platform Security: app code signing](https://support.apple.com/guide/security/sec7c917bf14/web): firma obligatoria y prevención de modificación del código.
