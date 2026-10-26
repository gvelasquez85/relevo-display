# Auditoría de eventos táctiles para Relevo Windows

Fecha: 2026-10-24. Responsable: Renzo Raines (kipp). Ejecución: `relevo-win-parity-20261024`.

## Alcance y evidencia

Inspección estática de evidencia retenida; no se capturaron eventos reales, no se inyectó entrada y no se modificó el iPad. Equipo de referencia comunicado por German: iPad 2 con iOS 9.3.5. El texto comercial proporcionado es una lista de funciones por contrastar, no prueba de paridad de nuestra implementación.

Fuente principal: `analysis/protocol-events.json`, `normalized_result`, procedimiento `-[CaptureController handleEventFromPeer:]` en `0x1000491e4`. Hopper sobre la aplicación Mac instalada, arquitectura arm64, SHA-256 `d308c40c0882dce2f460d957e79d0de1375d2099f2b367d5c8217eb141aae686`, evidencia `ev_225020f6f82bbe2ae1559857d0dd0b3a3437f5a69137a2efc32b44c25af6a20a`. Pseudocódigo y ensamblador son representaciones del descompilador, no código fuente original.

`analysis/protocol-receive.json`, procedimiento `0x1000538e4`, muestra deserialización mediante `dictionaryWithContentsOfDispatchData:` y entrega a `handleEventFromPeer:`. El dispatcher lee la clave `eventName`.

## Lo confirmado estáticamente

Las referencias son direcciones de llamada del ensamblador retenido. Los números de línea citados corresponden a `normalized_result.pseudocode.splitlines()`, pues el JSON original está serializado en una sola línea.

| Evento | Campos leídos en el dispatcher | Destino y llamada | Límite de la evidencia |
|---|---|---|---|
| `SingleTap` | `x`, `y` mediante `floatValue`; `mouseButton` mediante `integerValue`; `modifierFlags` opcional mediante entero/unsigned | `handleTapEvent:` en `0x1000497f0`; `handleKeyboardModifier:` en `0x1000497c8` | No prueba unidades, origen, rango ni botón. Pseudocódigo líneas 265–315. |
| `TwoFingerSingleTap` | `x`, `y` floats | `handleTwoFingerTapEvent:` en `0x100049880` | Clic secundario es intención del texto comercial, no semántica de Windows demostrada. Líneas 320–336. |
| `SingleTap2nd` | `x`, `y` floats | `handleTapSecondaryEvent:` en `0x1000499e0` | El nombre NO basta para asumir doble clic. Líneas 377–393. |
| `Scrolled` | Ningún campo adicional: entrega el diccionario completo | `handleScrollEvent:` en `0x1000498b4` | Deltas, ejes, signo, fase y unidad desconocidos. |
| `Dragged` | Diccionario completo | `handleDragEvent:` en `0x1000498e8` | Coordenadas, estado down/move/up/cancel y botón desconocidos. |
| `HoverEvent` | Diccionario completo | `handleHoverEvent:` en `0x100049950` | Movimiento absoluto o relativo y escala desconocidos. |
| `KeyboardEvent` | Diccionario completo | `handleKeyboardEvent:` en `0x100049a7c` | Keycode, texto, modificadores, repetición y liberación desconocidos. |
| `KeyboardPressEvent` | Diccionario completo | `handleKeyboardPressEvent:` en `0x100049ab0` | No asumir equivalencia con el anterior. |
| `vKeyboardEvent` | Diccionario completo | `handleVirtualKeyboardEvent:` en `0x100049ae4` | Texto/tecla e interpretación desconocidos. |
| `vKeyboardSourceEvent` | Diccionario completo | `handleVirtualKeyboardSourceEvent:` en `0x100049b1c` | No prueba entrada utilizable. |
| `Zoomed` | Diccionario completo | `handleZoomEvent:` en `0x100049a14` | Factor, ancla y fase desconocidos. |
| `TwoFingerPanned` | Diccionario completo | `handleTwoFingerPanEvent:` en `0x100049a48` | No asumir que es scroll. |
| `TwoFingerRotated` | Diccionario completo | `handleTwoFingerRotationEvent:` en `0x100049b50` | Ángulo, unidades y fase desconocidos. |
| `SwipeToChangeSpaceEvent` | Diccionario completo | `handleSwipeToChangeSpaceEvent:` en `0x10004991c` | Función macOS; adaptación a escritorios de Windows es una decisión de producto. |
| `DeviceResize` | `width`, `height` floats | Se guarda tamaño y llama `handleResizeEvent:` | No prueba si son puntos UIKit o píxeles. Líneas 23–42. |

Para los tres eventos que extraen x/y, el ensamblador mueve los valores convertidos a double a `d0` y `d1` justo antes de la llamada. Esto confirma un par de coordenadas flotantes; no una normalización particular. El pseudocódigo oculta estos argumentos de coma flotante y muestra un `r2` residual: no usarlo como supuesto parámetro del botón.

En `SingleTap`, la lectura/aplicación de `modifierFlags` está condicionada por `mouseButton != 0`; además comprueba presencia y el centinela `-1`. Esto no establece qué valor representa cada botón ni cómo traducir los bits macOS a Windows.

Los cuerpos `AppDelegate handleScrollEvent:`, `handleDragEvent:`, `handleHoverEvent:` y `handleKeyboardEvent:` **no están en los dossiers retenidos**. Las llamadas `_objc_msgSend$...` son stubs de dispatch, no los cuerpos receptores. Se recorrieron las entradas de `analysis/installed-2.4.8.1.rea.json` y `analysis/trial-protocol.rea.json`; ambos incluyen el dispatcher, pero no estos handlers.

## Qué implementar ahora

Confirmado para implementación determinista: parser tipado y acotado de `eventName`, validación de números finitos para x/y/width/height y contadores por nombre. Diagnóstico de esquema (claves y tipos), sin contenido de teclado ni valores de uso cotidiano. Es posible mostrar que llegan toques, pero **no habilitar inyección por defecto con una escala supuesta**.

Pendiente antes de entrada real: obtener los cuerpos de los handlers o capturar una prueba acotada de gestos con patrón público y coordenadas conocidas. Es necesario demostrar origen, escala y transformación por orientación, resolución, letterbox y posición de pantalla virtual. Para drag hay que demostrar liberación y cancelación; para teclado, tabla de códigos y estados. No reutilizar directamente flags/códigos macOS en `SendInput`.

## Validación mínima propuesta

1. Mostrar una cuadrícula de prueba en 1024×768, tocar centro y cuatro esquinas, repetir en otra resolución y orientación. Registrar sólo esos puntos del patrón. Aceptación: la transformación identifica sin ambigüedad unidades/origen y sitúa cada toque en la celda seleccionada.
2. Ejecutar toque, doble toque, dos dedos, hover, scroll en ambos ejes y drag completo/cancelado. Aceptación: secuencia distinguible y no queda botón pulsado al desconectar o cancelar.
3. Activar entrada Windows sólo tras prueba con pantalla virtual identificada. Aceptación: no hay clic en otra pantalla por cambio de topología; libera estados al parar, perder conexión o entrar al escritorio seguro; nunca intenta interactuar con UAC.
4. Teclado después de lo anterior, con texto artificial. Aceptación: pulsación/liberación, modificadores y caracteres verificados; sin guardar texto introducido.

## Fuera del alcance del iPad de referencia

Drag/drop entre apps anunciado para iOS 11+ no está disponible en iOS 9.3.5. Split View no debe prometerse en iPad 2; no se prueba por ver la ruta `DeviceResize`. Touch Bar, AirPlay y Spaces son funciones macOS y requieren adaptación o descarte explícito para Windows. No se infiere soporte real del cliente iOS antiguo por la presencia de eventos en un servidor Mac reciente.

## Diagnóstico de esquema acotado (Relevo, ejecución relevo-next-20261024)

`session.InputEventSchema` acepta únicamente los 15 nombres de entrada de la tabla anterior. Devuelve como máximo diez claves fijas: `eventName`, `x`, `y`, `width`, `height`, `mouseButton`, `modifierFlags`, `deltaX`, `deltaY`, `phase`. Los valores devueltos son etiquetas estáticas de tipo (`string`, `number`, `boolean`, `null`, `object`, `array`, `other`), nunca el contenido original. No recorre objetos ni listas y descarta nombres de campo desconocidos, que también podrían contener texto privado.

Si hay coordenadas, dimensiones o deltas presentes, exige números finitos; valores ausentes siguen permitidos para observar esquemas incompletos. Rechaza eventos desconocidos o inválidos sin devolver un esquema. Esto valida estructura, no unidades, rangos físicos, orientación ni semántica de gestos. No habilita inyección de entrada Windows. El consumidor debe limitar también cuántos esquemas conserva y nunca registrar el diccionario original ante un rechazo.

Verificación: pruebas unitarias con `go test -race ./internal/session -run TestInputSchema`, incluyendo rechazo de NaN/infinito/texto en campos numéricos, nombres desconocidos y comprobación de ausencia de texto privado/valores en el esquema serializado. No se capturaron gestos del iPad en esta tarea.
