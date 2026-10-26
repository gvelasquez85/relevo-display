# Desarrollo

El repositorio contiene un solo producto y una entrega vigente de Windows. Las pruebas de uso se hacen desde el GUI de Relevo; no se distribuyen laboratorios, scripts ni paquetes de prueba separados al usuario.

Antes de cambiar funciones, revisar [el índice de documentación](docs/README.md), [la arquitectura](docs/architecture.md) y las instrucciones de [AGENTS.md](AGENTS.md).

## Verificación interna

Para desarrolladores: Go 1.23 o posterior, Python 3 y compilador C/C++17 para las pruebas portables de medios. `scripts/build.sh` ejecuta pruebas, análisis estático, compilación de Windows y empaquetado único. Los SDK de Apple y Windows/WDK se suministran por separado y no se versionan. El cliente ARMv7 se construye con `clients/ios/build-with-sdk.sh` y un SDK oficial compatible.

Compilar para un sistema no acredita funcionamiento físico. Registrar por separado pruebas automatizadas, compilación cruzada, pruebas de escritorio y pruebas reales con iPad. El pipeline CI verifica código portable; no instala dispositivos virtuales ni firma apps iOS.

No subir herramientas descargadas, dist, SDKs, certificados, perfiles de aprovisionamiento, tokens, registros del dispositivo, capturas ni claves privadas. Conservar las licencias y la procedencia de componentes de terceros. No modificar paquetes de controladores firmados para cambiar su identidad.
