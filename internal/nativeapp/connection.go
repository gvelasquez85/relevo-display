package nativeapp

import (
	"context"
	"fmt"
	"net"
	"relevo/internal/lantransport"
	"relevo/internal/usbmux"
)

func openConnection(ctx context.Context, cfg trialConfig, report func(string)) (net.Conn, func(), error) {
	transport := cfg.Transport
	if transport == "" {
		transport = "USB"
	}
	if transport == "WiFi" {
		if cfg.Pairing == nil {
			return nil, nil, fmt.Errorf("abre el emparejamiento Wi-Fi desde el GUI de Relevo")
		}
		server, e := lantransport.New(cfg.LANAddress)
		if e != nil {
			return nil, nil, e
		}
		pairCtx, closePairing := context.WithCancel(ctx)
		dismissed, e := cfg.Pairing(pairCtx, server.PairingURI())
		if e != nil {
			closePairing()
			server.Close()
			return nil, nil, fmt.Errorf("no se pudo abrir el emparejamiento Wi-Fi")
		}
		acceptCtx, cancelAccept := context.WithCancel(ctx)
		watcherDone := make(chan struct{})
		go func() {
			select {
			case <-dismissed:
				cancelAccept()
			case <-watcherDone:
			case <-acceptCtx.Done():
			}
		}()
		report("Escanea el QR desde Relevo en el iPad. Ambos equipos deben estar en la misma red. El emparejamiento vence en 60 segundos.")
		conn, e := server.Accept(acceptCtx)
		close(watcherDone)
		cancelAccept()
		closePairing()
		if e != nil {
			server.Close()
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, fmt.Errorf("no se completó el emparejamiento Wi-Fi; vuelve a iniciar y escanea un QR nuevo")
		}
		report("iPad enlazado por Wi-Fi cifrado. Preparando pantalla y entrada…")
		return conn, func() { server.Close() }, nil
	}
	if transport != "USB" {
		return nil, nil, fmt.Errorf("conexión no compatible")
	}
	if e := localEndpoint(cfg.Endpoint); e != nil {
		return nil, nil, e
	}
	mux := usbmux.Client{Endpoint: cfg.Endpoint}
	ds, e := mux.Devices()
	if e != nil {
		return nil, nil, fmt.Errorf("no se pudo abrir el servicio USB de Apple: %w", e)
	}
	device, e := usbmux.Choose(ds, 0)
	if e != nil {
		return nil, nil, e
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	report("iPad detectado por USB. Conectando con el cliente Relevo…")
	conn, e := mux.Connect(device.ID, 52742)
	if e != nil {
		return nil, nil, fmt.Errorf("abre nuestra app Relevo verificada en el iPad e inicia USB: %w", e)
	}
	report("Enlace USB abierto. Preparando pantalla y funciones…")
	return conn, func() { conn.Close() }, nil
}
