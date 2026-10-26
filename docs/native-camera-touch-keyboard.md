# Relevo: cámara integrada, multitáctil y teclado

Requisitos actualizados: 25 octubre 2026. Diseño propuesto; funciones todavía no implementadas ni verificadas en equipos físicos.

## Cámara

Producto final sin OBS. Un único instalador Windows integra pantalla virtual, aplicación y proveedor de cámara Relevo. El receptor iOS propio envía cámara por USB; el host publica imágenes mediante una fuente Media Foundation. Windows 11 es el primer objetivo, usando MFCreateVirtualCamera y el ejemplo oficial Microsoft Windows-Camera/Samples/VirtualCamera. No es un controlador UVC listo para instalar: requiere implementar y registrar nuestra fuente COM, transporte local entre procesos y formatos negociados. El iPad actual no se presenta como webcam UVC.

La fuente debe estar disponible para Frame Server, con instalación en ruta protegida y acceso IPC limitado. Negociar formatos y resolución sin anunciar capacidades que la cámara no entrega. Cámara apagada sin consentimiento explícito; desconexión debe liberar captura y señalizar ausencia, sin conservar indefinidamente la última imagen. Desinstalación elimina registro y dispositivo propio. Validar Cámara de Windows, navegador, Teams y Zoom individualmente; ninguna compatibilidad se da por probada.

Windows 10 queda fuera de esta ruta API: requeriría evaluar por separado DirectShow o AVStream, firma, redistribución y compatibilidad. No introducir silenciosamente un driver de terceros ni desactivar verificaciones de firma.

## Entrada táctil

Estado v0.4: multipleTouchEnabled=NO, un dedo traducido a mouse. Nuevo alcance: identificadores estables por contacto, fase, posición normalizada y grupos de contactos sincronizados. Windows recibe touch real mediante InitializeTouchInjection/InjectTouchInput, conservando coordenadas de la pantalla extendida, DPI y orientación. No duplicar mouse y touch para un mismo gesto.

Dos modos seleccionables: pantalla táctil directa (Windows/aplicación interpreta contactos) y gestos de control (traducción explícita a scroll, clic derecho, navegación, zoom y escritorios). Tap, doble tap, pulsación larga/arrastre, scroll de uno/dos dedos, pinch, rotación, swipe y tres dedos para teclado deben tener pruebas independientes. La acción disponible depende de la aplicación Windows. Gestos reservados de iOS pueden interceptarse antes de llegar al cliente; no prometer capturarlos todos. Mantener iOS 9.3.5 e iPad 2 como plataforma mínima.

Desconexión, segundo plano o cambio de modo cancelan todos los contactos y liberan modificadores. Respetar límites del escritorio seguro/UAC y permisos del proceso receptor.

## Teclado

Mostrar teclado nativo iOS en el cliente propio; enviar texto Unicode confirmado y eventos separados para borrar, Enter, Tab, Escape, flechas y modificadores Ctrl/Alt/Win/Shift. No equivale a conectar físicamente un teclado USB. Contemplar español, ñ, tildes, composición y caracteres no BMP; evitar duplicación durante composición. Botón visible y gesto para mostrar/ocultar. Desconexión libera todas las teclas. No almacenar texto ni teclas en logs.

## Entregas y aceptación

1. Entrada: ampliar protocolo negociado, cliente y host de forma compatible; pruebas de contactos simultáneos, composición, cancelación y escritorio extendido.
2. Cámara: prueba mínima Windows 11 con imagen sintética, registro/enumeración y desinstalación; después conectar frames reales del iPad. Registrar latencia y comportamiento de desconexión.
3. Un instalador: incluir componentes propios y redistribuibles permitidos; comprobar instalación limpia sin OBS y desinstalación. La firma iOS sigue siendo requisito independiente para instalar el cliente.

## Fuentes primarias revisadas

- https://learn.microsoft.com/en-us/windows/win32/api/mfvirtualcamera/nf-mfvirtualcamera-mfcreatevirtualcamera
- https://github.com/microsoft/Windows-Camera/tree/master/Samples/VirtualCamera
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-injecttouchinput
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput

## Avance independiente de Xcode

Implementado `internal/remoteinput`: hasta diez contactos simultáneos, validación atómica de lotes, identificadores estables, coordenadas finitas normalizadas y cancelación de todos los contactos. Teclado con texto Unicode confirmado, teclas de control y liberación de teclas sostenidas. Pruebas incluyen dos contactos que se mueven de forma independiente, rechazo de lotes incompletos sin aplicación parcial, coordenadas fuera de imagen, español/caracteres no BMP y modificadores sostenidos al desconectar.

Este módulo aún no se negocia por RVL1 ni está conectado a UIKit o InjectTouchInput/SendInput. No cambia el ejecutable entregado ni constituye soporte multitáctil físico validado. Próximos pasos: negociación de capacidades, integración de contactos UIKit e inyección Windows, luego teclado nativo y pruebas de gestos por aplicación.
