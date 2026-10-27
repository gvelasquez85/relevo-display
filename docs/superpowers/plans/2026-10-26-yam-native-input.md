# Yam USB: gestos Windows y campos de navegador — 0.15.2

Requerimiento: ampliar multitouch; descartar distribución del teclado propio; preferencia por teclado iOS y apertura en barra de direcciones del navegador.

Hechos: usuario confirmó escritura Notepad con 0.15.1. Probe anterior sólo admitía Edit/RichEdit Win32 y excluía Chrome/Edge. El dispatcher Mac inspeccionado presenta eventos agregados de pan/zoom; no demuestra contactos individuales. La documentación oficial https://www.yamdisplay.com/about/ restringe el gesto de apertura del teclado en pantalla a Yam Air. No se encontró una orden de apertura iOS en el servidor USB inspeccionado. No se demuestra imposibilidad absoluta, pero no hay ruta verificada con Yam USB instalado. USB local no disponible durante investigación.

Cambios: ProbeEditable añade UIAutomation aislado en proceso del mismo EXE con límite de 500ms; sólo metadatos, sin texto/URL/valores. Se mantiene Probe para ruta nativa de baja latencia. Teclado propio retirado de ruta normal, OSK estándar Windows como alternativa provisional integrada (no teclado iOS). Botón manual y apertura contextual; no se relanza teclado visible. Sólo administra ventana de OSK cuya propiedad del proceso pueda verificarse.

Modo opcional GUI «Gestos de Windows (dos dedos)»: reconstruye dos contactos desde TwoFingerPanned/Zoomed; start/update/end, cancelación al perder geometría/error/ambigüedad/inactividad. No son trayectorias individuales originales ni soporte de diez dedos; modo anterior conservado por defecto.

Verificación: unit/race de geometría, lifecycle, contactos, rechazo de readonly/foco ajeno, persistencia; vet portable/Windows y compilación Windows. Pendientes: barra Chrome/Edge real, ventana OSK/UIAccess y gestos en aplicaciones Windows desde iPad. No equivalen a validación física.
