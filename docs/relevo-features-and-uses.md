> Actualización posterior del usuario, 24 de octubre de 2026: el dispositivo real tiene **iOS 9.3.5**, y ese mínimo es suficiente. Las referencias siguientes a iOS 7 y a soporte obligatorio de iOS 5.1.1 describen el supuesto anterior y quedan sustituidas para la siguiente implementación.

# Relevo Display: funciones y usos después del monitor

24 octubre 2026. Responsable: Murdock, encargo de Hannibal `relevo-phase2-analysis-20261024`. Documento de análisis; no implementa funciones ni modifica dispositivos.

## Punto de partida y propuesta de valor

El equipo de referencia es **iPad 2 con iOS 7 conectado a Windows 11**. German confirma que la versión 0.6 funciona mucho mejor, Windows reconoce la pantalla y sigue percibiendo algo de lag. Es una validación humana de uso; no aporta FPS visibles o latencia medidos. El producto pretende aprovechar ese equipo que ya tiene como monitor y, después, como superficie de control. No existe todavía evidencia de demanda de otros clientes ni de todas las capacidades sugeridas.

La fase 1 tiene dos criterios distintos: fluidez suficiente para German y eliminación de la dependencia del receptor de terceros. Renombrar el servidor no elimina esa dependencia. Un cliente propio instalado y probado en el iPad es necesario para una experiencia independiente; los avisos legales de dependencias que permanezcan no se eliminan como parte del cambio de identidad. La revisión de licencias corresponde al frente específico, no a este análisis.

Evidencia local: [README](../README.md), [evaluación del códec](codec-evaluation-2026-10-24.md), [ideas originales](ideas-futuras-ipad.md), `internal/session/session.go` (estadísticas del emisor). El lector debe distinguir esas evidencias de las hipótesis siguientes.

## Prioridad propuesta

La viabilidad indicada es una evaluación provisional para iPad 2/iOS 7, no soporte certificado. Esfuerzo relativo: bajo/medio/alto respecto del monitor actual; no son estimaciones de calendario.

| Orden / función | Valor y problema | Dependencias / esfuerzo | Viabilidad antigua y evidencia | Experimento mínimo y decisión |
|---|---|---|---|---|
| 1. Fluidez, reconexión y diagnóstico | Quitar el lag aún informado y evitar reinicios manuales | Instrumentación por etapa, control de colas y recuperación; medio | Monitor demostrado por German; cuello de botella pendiente | Comparar cursor, scroll y vídeo en el mismo equipo. Continuar sólo con mejora visible y estabilidad; no aceptar únicamente más FPS enviados |
| 2. Touch, arrastre, scroll y teclado | Manejar ventanas desde el iPad sin ratón adicional | Decodificar eventos, mapear coordenadas/orientación y entrada Windows; medio | Eventos identificados en protocolo; semántica completa y entrada no probadas | Toque en nueve puntos, arrastre entre esquinas, scroll y escritura; verificar que no hay clics en la pantalla equivocada ni teclas retenidas tras desconexión |
| 3. Perfiles y preparación simple | Conectar y trabajar sin configurar cada sesión | Recordar resolución/posición/escala, instalador y mensajes de estado; medio | GUI y ajustes de 0.6 existentes; experiencia integral no medida | Inicio desde instalación limpia y reconexión; usuario completa sesión y recupera un fallo sin BAT ni conocimiento técnico |
| 4. Monitor principal y control de Raspberry Pi | Usar un Pi sin monitor, teclado ni ratón físicos | Sesión gráfica sin pantalla física, servicio de inicio, captura e inyección Linux, transporte; alto | Objetivo del usuario; funcionamiento completo sin monitor no probado | Arranque en frío de un modelo/OS identificados; abrir escritorio, terminal y reiniciar desde iPad. Continuar con tres arranques consecutivos completos |
| 5. Extensión Linux | Reutilizar iPad con Ubuntu, Omarchy y Bazzite | Validación específica del compositor, empaquetado y entrada; alto | Backends experimentales existentes; ninguna validación integral en esas distribuciones | Un entorno cada vez: crear monitor, mover ventana, desconectar y restaurar; bloquear publicación de soporte hasta probar cada combinación |
| 6. Audio hacia altavoces del iPad | Evitar altavoz adicional para el equipo anfitrión | Cliente propio receptor, captura/salida de audio anfitrión, sincronía y permisos; alto | Idea; canal de audio no confirmado en receptor actual | Tono y voz en sesión de pantalla de diez minutos; medir cortes y desfase audiovisual antes de añadir dispositivo virtual permanente |
| 7. Cámara como webcam | Reutilizar cámara para llamadas | Cliente propio, captura de cámara, transporte de vuelta, cámara virtual Windows y permisos; alto | Idea; no existe canal demostrado | Mostrar una cámara en una app Windows identificada; después probar vídeo y pantalla simultáneos. Pausar si perjudica el monitor o no alcanza utilidad para German |
| 8. Audio hacia auriculares conectados al iPad | Escuchar el anfitrión a través de la salida elegida en iPad | Primero audio propio; luego compatibilidad de accesorio y enrutamiento real; alto | AirPods con iOS 7 no confirmados; no prometer soporte | Probar primero un accesorio/modelo concreto en iPad; después cambio de ruta, desconexión y recuperación. No avanzar si no funciona la salida local |
| 9. Bluetooth u otras apps iOS | Idea aún sin tarea concreta | Definir acción, APIs y apps compatibles; indeterminado | USB no demuestra un puente transparente de Bluetooth ni control general de iOS | Elegir una acción acotada, por ejemplo abrir una app que admita enlace. No diseñar un hub Bluetooth ni prometer ejecutar/controlar cualquier app |

## Usos que conviene validar por separado

- **Monitor auxiliar de escritorio:** trabajo que ya realiza German. Debe demostrar valor al conectar el cable y mover una ventana; el primer criterio es que el cursor y el scroll se sientan utilizables.
- **Terminal de Raspberry Pi:** iPad como pantalla principal y entrada. Debe funcionar después de arrancar el Pi sin otro monitor; duplicar una pantalla física no demuestra este caso.
- **Panel de controles:** botones grandes para acciones explícitas del equipo anfitrión. Es una hipótesis útil si el monitor/touch ya funcionan; probar primero tres acciones que German use, sin construir un catálogo de integraciones.
- **Control táctil adaptado:** tamaño de objetivos, gestos configurables y alternativa a arrastre pueden ayudar a algunas personas. Requiere pruebas con necesidades concretas, teclado y foco. No equivale a certificación de accesibilidad ni a promesa de utilidad universal.

No lanzar estos usos como productos distintos todavía. Comparten transporte y pantalla, pero el arranque sin monitor, la entrada y la instalación son criterios diferentes.

## Cierre de fluidez de la fase 1

Cambiar JPEG por H.264 puede reducir tráfico, pero también puede añadir buffering. Revisar **captura → conversión → codificación → cola → USB → decodificación → presentación**. La versión actual registra medidas del emisor; no conoce la presentación real del receptor. No atribuir el lag únicamente al códec ni interpretar `sender_fps` como refresco del iPad.

Prueba propuesta: mismo Windows 11/iPad 2/iOS 7, 1024×768, tres repeticiones de un minuto de cursor, scroll y vídeo por configuración. Registrar p50/p95 de etapas disponibles, CPU, tráfico, errores y una grabación externa para estimar latencia visible. Comparar primero JPEG optimizado y después H.264 sólo cuando el encapsulado se valide.

Como referencia **por acordar**, el documento de códec propone ≥25 FPS visibles y latencia visible p95 ≤150 ms. No son resultados ni requisitos aprobados por German. El cierre requiere también su aceptación de fluidez en uso real y recuperación después de UAC/desconexión sin perder la configuración. Si un códec falla, conservar la ruta estable y corregir la etapa lenta.

## Recomendación de alcance

Mantener un solo objetivo inmediato: **monitor fluido y controlable en iPad 2/iOS 7/Windows 11**. En paralelo, resolver la prueba de instalación y ejecución del cliente propio; esa prueba condiciona audio/cámara y la independencia del receptor actual. Después abordar Pi sin monitor y Linux por entorno. Mantener audio y webcam en exploración, sin añadirlos al instalador ni al flujo inicial ahora.

No se realizaron pruebas de hardware, entrevistas adicionales, mediciones de latencia ni investigación de mercado en este encargo. Las funciones nuevas y sus umbrales siguen siendo propuestas; no se promete soporte de AirPods, Bluetooth general, cámara o control arbitrario de apps.
