# Documentación de Relevo Display

El [README principal](../README.md) contiene el estado vigente. Esta carpeta reúne documentación de desarrollo; las pruebas del usuario se presentan dentro del GUI de Windows, en la única entrega actual.

## Punto de entrada

- [Arquitectura y desarrollo](architecture.md): componentes, plataformas, protocolos y límites de validación.
- [Necesidades del cliente propio](relevo-native-client-requirements.md): requisitos del iPad y comportamiento esperado.
- [Funciones y usos](relevo-features-and-uses.md) e [ideas futuras](ideas-futuras-ipad.md): propuestas; no equivalen a funciones disponibles.
- [Identidad del producto](product-identity.md): nombre Relevo Display.

## Decisiones técnicas

- [Pantalla virtual en Windows](../WINDOWS-VIRTUAL.md) y [comparación de funciones](../FEATURES-WINDOWS.md): antecedentes técnicos de la ruta compatible.
- [Evaluación de códecs](codec-evaluation-2026-10-24.md): investigación de rendimiento y límites de las mediciones.
- [Touch, teclado y cámara propios](native-camera-touch-keyboard.md).
- [Dispositivo de micrófono virtual](microphone-virtual-device.md) y [componentes multimedia de Windows](../drivers/windows-media/README.md): implementación parcial y requisitos pendientes.
- [Conexión Wi-Fi](wifi-connection-design.md): diseño propuesto; no habilita sesiones inalámbricas.
- [Toolchain iOS 9.3.5](ios935-toolchain-feasibility.md) y [opciones de instalación](relevo-installation-options.md): investigación de compilación y firma. Firmar e instalar no acredita que iOS permita ejecutar la app.

## Historial

Los documentos `native-v*-test-guide.md`, `native-gui-test-guide.md` y los planes de [`superpowers`](superpowers) describen revisiones y pruebas internas anteriores. Sus nombres de paquetes, comandos y afirmaciones de estado pueden haber sido superados. No deben usarse para publicar entregas separadas: la política actual es extender el mismo GUI y mantener únicamente `Relevo-Windows.exe` y su ZIP como entrega vigente.

Cuando cambie una capacidad, actualiza su estado y evidencia: código disponible, compilación, prueba automatizada, instalación y validación física son hitos distintos. No incluyas credenciales, perfiles de firma, identificadores del dispositivo ni registros privados en la documentación del repositorio.
