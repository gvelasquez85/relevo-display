# Corrección basada en iPad real — 0.15.3

Yam2.2/iPad2,1/iOS9.3.5 USB. Reproducción: zoom visible en iPad, cero eventos Zoomed recibidos en host. Añadir initZoomMode:1 y zoomMode:1, órdenes observadas en servidor oficial, hizo llegar Zoomed con centro, scale y fases. Nueva opción interna AppGestures en session, activada cuando entrada touch Windows está disponible. Si entrada no disponible se conserva zoom local del receptor. No se modifica el binario del cliente ni su licencia.

Correcciones acompañantes: anclar pinch en x/y recibidos al iniciar; mapear Scrolled touchType3 a pan de dos contactos en modo nativo, según handler Mac; diagnóstico permite tipo de touchType sin valores. Pruebas reproducen previamente ausencia de orden, centro incorrecto y scroll convertido en rueda.

Hechos pendientes del requisito original: tres dedos no abrió teclado iOS en prueba real. No hay evento de contactos independientes en lo capturado. No se presenta reconstrucción como multitouch crudo. Copia ApplicationOnly+SkipUninstall rechazada por iOS como UnknownCommand; app sigue instalada. Aether no accesible mediante automatización; no se atribuye análisis al programa. Servidor Mac restaurado.

Artefacto de evidencia interna: analysis/yam-live-input-20261027.json. Validación USB de recepción; falta validar inyección física Windows.
