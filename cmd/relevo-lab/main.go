// Relevo Lab is an independent foreground browser diagnostic, not a display host.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"relevo/internal/medialab"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8765", "dirección local y puerto")
	lan := flag.Bool("lan", false, "habilitar acceso desde la red local (enlace temporal sin cifrar)")
	flag.Parse()
	interactive := runtime.GOOS == "windows" && len(os.Args) == 1
	if interactive {
		fmt.Println("Relevo Lab: audio y foto por Wi-Fi privada. Safari puede pausar Yam.")
		fmt.Println("Pulsa Enter para iniciar la prueba en red local; Ctrl+C para cancelar.")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			return
		}
		*lan = true
	}
	host, port, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatal("Dirección inválida: ", err)
	}
	if !*lan {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			log.Fatal("Usa -lan para habilitar una dirección fuera de loopback")
		}
	} else {
		host = "0.0.0.0"
	}
	handler, token, err := medialab.New()
	if err != nil {
		log.Fatal("No se pudo generar la clave temporal")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Fatal("No se pudo abrir el puerto: ", err)
	}
	actualPort := fmt.Sprint(listener.Addr().(*net.TCPAddr).Port)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	fmt.Println("Relevo Lab: tono WAV y foto estática manual por red local; no captura escritorio ni vídeo.")
	fmt.Printf("En este computador: http://127.0.0.1:%s/%s/\n", actualPort, token)
	if interactive {
		openBrowser(fmt.Sprintf("http://127.0.0.1:%s/%s/", actualPort, token))
	}
	if *lan {
		urls, err := medialab.PrivateURLs(actualPort, token)
		if err != nil || len(urls) == 0 {
			fmt.Println("No se encontró una dirección IPv4 privada para el iPad.")
		}
		for _, u := range urls {
			fmt.Println("Safari del iPad: ", u)
		}
		fmt.Println("Misma Wi-Fi privada. Puede ser necesario permitir el puerto en el firewall de Windows; no se cambia automáticamente.")
	}
	fmt.Println("Mantén privado el enlace. HTTP local sin cifrar. Ctrl+C para cerrar.")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			log.Print("El servidor terminó: ", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
	}
}
