//go:build windows

// Package wintray provides the Windows notification-area application shell.
package wintray

import (
	"context"
	"fmt"
	"os"
	"relevo/internal/lantransport"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type Options struct {
	PrepareStop       func() error
	OpenKeyboard      func() error
	DisconnectMonitor func() error
	OpenYamTests      func(Settings)
	OpenTests         func()
	ExportDiagnostic  func() error
	Title             string
	// Start blocks until stopped or the display session ends. It runs off the UI thread.
	Start   func(settings Settings, stop <-chan struct{}, report func(string)) error
	Install func() error
}

var user = syscall.NewLazyDLL("user32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")

func p(s string) *uint16                  { return syscall.StringToUTF16Ptr(s) }
func call(n string, a ...uintptr) uintptr { r, _, _ := user.NewProc(n).Call(a...); return r }
func message(hwnd uintptr, text string) {
	call("MessageBoxW", hwnd, uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Relevo Display"))), 0x40)
}
func Open(target string) error {
	r, _, e := shell.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(p("open"))), uintptr(unsafe.Pointer(p(target))), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("open %s: %v (code %d)", target, e, r)
	}
	return nil
}

// InstallElevated starts this executable through the standard Windows UAC prompt.
type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance                          uintptr
	IDList                            uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon, Process                     uintptr
}

func InstallElevated() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	info := shellExecuteInfo{Mask: 0x40, Verb: p("runas"), File: p(exe), Parameters: p("--install-driver --gui-install"), Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, e := shell.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return fmt.Errorf("iniciar instalador: %v", e)
	}
	if info.Process == 0 {
		return fmt.Errorf("Windows no devolvió el proceso del instalador")
	}
	defer syscall.CloseHandle(syscall.Handle(info.Process))
	k := syscall.NewLazyDLL("kernel32.dll")
	r, _, e = k.NewProc("WaitForSingleObject").Call(info.Process, 0xffffffff)
	if r != 0 {
		return fmt.Errorf("esperar instalador: %v", e)
	}
	var code uint32
	r, _, e = k.NewProc("GetExitCodeProcess").Call(info.Process, uintptr(unsafe.Pointer(&code)))
	if r == 0 {
		return e
	}
	if code != 0 {
		return fmt.Errorf("el instalador terminó con código %d", code)
	}
	return nil
}

func ShowError(err error) {
	if err != nil {
		message(0, err.Error())
	}
}
func ShowResult(err error) {
	s := "Controlador preparado. Reinicia Windows para que anuncie las nuevas resoluciones 4:3 del iPad. Después abre Relevo Display y usa el icono junto al reloj para iniciar la extensión."
	if err != nil {
		s = err.Error()
	}
	message(0, s)
}

type point struct{ X, Y int32 }
type msg struct {
	H       uintptr
	M       uint32
	W       uintptr
	L       uintptr
	Time    uint32
	P       point
	Private uint32
}
type wc struct {
	Size, Style                        uint32
	Proc                               uintptr
	Extra, WinExtra                    int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type nid struct {
	Size                uint32
	H                   uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	BalloonIcon         uintptr
}

// monitorIcon creates a small branded monitor instead of the generic Windows icon.
func monitorIcon(instance uintptr) uintptr {
	const n = 32
	and := make([]byte, n*n/8)
	xor := make([]byte, n*n*4)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			i := (y*n + x) * 4
			on := x >= 3 && x < 29 && y >= 5 && y < 25 || x >= 14 && x < 18 && y >= 25 && y < 28 || x >= 10 && x < 22 && y >= 28 && y < 30
			if !on {
				and[(y*n+x)/8] |= 1 << uint(7-x%8)
				continue
			}
			xor[i], xor[i+1], xor[i+2] = 60, 45, 25
			if x >= 5 && x < 27 && y >= 7 && y < 23 {
				xor[i], xor[i+1], xor[i+2] = 180, 180, 30
				if x > 16 {
					xor[i], xor[i+1], xor[i+2] = 215, 105, 35
				}
			}
		}
	}
	return call("CreateIcon", instance, n, n, 1, 32, uintptr(unsafe.Pointer(&and[0])), uintptr(unsafe.Pointer(&xor[0])))
}

func Run(o Options) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	k := syscall.NewLazyDLL("kernel32.dll")

	if o.Title == "" {
		o.Title = "Relevo Display 0.9"
	}
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 20*time.Second)
	singleton, existing, startupErr := acquireShell(startupCtx, o.Title, windowsSingletonAPI{})
	cancelStartup()
	if startupErr != nil {
		return startupErr
	}
	if existing {
		return nil
	}
	defer syscall.CloseHandle(syscall.Handle(singleton))
	settings, loadErr := LoadSettings()
	var hwnd, status, resCombo, fpsCombo, qualityCombo, modeCombo, positionCombo, awakeCheck, touchCheck, keyboardCheck, startButton, stopButton, installButton, saveButton, testsButton, yamTestsButton uintptr
	var clientCombo, transportCombo, lanCombo, nativeGesturesCheck, autoStartCheck uintptr
	var lanAddresses = lantransport.Addresses()
	var active, installing, disconnecting, closing, testsOpen bool
	var stop chan struct{}
	var mu sync.Mutex
	var result error
	var progressText string
	var stopGuard stopPreflight
	var stopNotice string
	type stopCheckResult struct {
		token uint64
		err   error
	}
	stopChecks := make(chan stopCheckResult, 1)
	const callback = 0x8001
	const finished = 0x8002
	const installed = 0x8003
	const progress = 0x8004
	const disconnected = 0x8005
	const stopPrepared = 0x8006
	instance, _, _ := k.NewProc("GetModuleHandleW").Call(0)
	hi := monitorIcon(instance)
	if hi == 0 {
		return fmt.Errorf("no se pudo crear el icono")
	}
	defer call("DestroyIcon", hi)
	var icon nid
	notify := func(op uintptr) { shell.NewProc("Shell_NotifyIconW").Call(op, uintptr(unsafe.Pointer(&icon))) }
	setText := func(h uintptr, s string) { call("SetWindowTextW", h, uintptr(unsafe.Pointer(p(s)))) }
	update := func() {
		text := "Listo. Conecta el iPad por USB y abre la app compatible."
		if active {
			text = "Conectando con el iPad…"
		}
		if installing {
			text = "Preparando controlador…"
		}
		if stopGuard.pending {
			text = "Comprobando otra salida de pantalla antes de detener…"
		}
		if disconnecting {
			text = "Desconectando dispositivo de pantalla… Windows puede solicitar permisos de administrador."
		}
		setText(status, text)
		enabled := uintptr(1)
		if active || installing || disconnecting {
			enabled = 0
		}
		for _, h := range []uintptr{startButton, installButton, resCombo, fpsCombo, qualityCombo, modeCombo, positionCombo, awakeCheck, touchCheck, keyboardCheck, saveButton, testsButton, yamTestsButton, clientCombo, transportCombo, lanCombo, nativeGesturesCheck} {
			call("EnableWindow", h, enabled)
		}
		if enabled != 0 {
			clientIndex := call("SendMessageW", clientCombo, 0x147, 0, 0)
			transportEnabled := uintptr(0)
			if clientIndex == 1 {
				transportEnabled = 1
				setText(keyboardCheck, "Teclado iPad")
			} else {
				setText(keyboardCheck, "Teclado Windows")
			}
			call("EnableWindow", nativeGesturesCheck, 1-transportEnabled)
			call("EnableWindow", transportCombo, transportEnabled)
			lanEnabled := uintptr(0)
			if clientIndex == 1 && call("SendMessageW", transportCombo, 0x147, 0, 0) == 1 {
				lanEnabled = 1
			}
			call("EnableWindow", lanCombo, lanEnabled)
		}
		en := uintptr(0)

		mode := settings.Mode
		if modeCombo != 0 {
			index := call("SendMessageW", modeCombo, 0x147, 0, 0)
			if index == 0 {
				mode = "extend"
			} else if index == 1 {
				mode = "desktop"
			}
		}
		if !installing && !disconnecting && !closing && !stopGuard.pending && ((active && stop != nil) || (!active && mode == "extend" && o.DisconnectMonitor != nil)) {
			en = 1
		}
		call("EnableWindow", stopButton, en)
		for i := range icon.Tip {
			icon.Tip[i] = 0
		}
		copy(icon.Tip[:], syscall.StringToUTF16(o.Title+" · "+settings.Resolution+" · "+text))
		notify(1)
	}
	read := func() Settings {
		s := settings
		r := int(call("SendMessageW", resCombo, 0x147, 0, 0))
		f := int(call("SendMessageW", fpsCombo, 0x147, 0, 0))
		q := int(call("SendMessageW", qualityCombo, 0x147, 0, 0))
		if r >= 0 && r < len(Resolutions) {
			s.Resolution = Resolutions[r]
		}
		if f >= 0 && f < len(FrameRates) {
			s.FPS = FrameRates[f]
		}
		if q >= 0 && q < len(JPEGQualities) {
			s.Quality = JPEGQualities[q]
		}
		m := int(call("SendMessageW", modeCombo, 0x147, 0, 0))
		if m >= 0 && m < 2 {
			s.Mode = []string{"extend", "desktop"}[m]
		}
		p := int(call("SendMessageW", positionCombo, 0x147, 0, 0))
		if p >= 0 && p < 4 {
			s.Position = []string{"right", "left", "above", "below"}[p]
		}
		s.KeepAwake = call("SendMessageW", awakeCheck, 0xf0, 0, 0) == 1
		ci := call("SendMessageW", clientCombo, 0x147, 0, 0)
		if ci == 1 {
			s.Client = "relevo"
		} else {
			s.Client = ""
		}
		ti := call("SendMessageW", transportCombo, 0x147, 0, 0)
		if ci == 1 && ti == 1 {
			s.Transport = "WiFi"
		} else {
			s.Transport = ""
		}
		li := int(call("SendMessageW", lanCombo, 0x147, 0, 0))
		if li >= 0 && li < len(lanAddresses) {
			s.LANAddress = lanAddresses[li]
		}
		s.Touch = call("SendMessageW", touchCheck, 0xf0, 0, 0) == 1
		s.Keyboard = call("SendMessageW", keyboardCheck, 0xf0, 0, 0) == 1
		s.AutoStart = call("SendMessageW", autoStartCheck, 0xf0, 0, 0) == 1
		s.NativeGestures = call("SendMessageW", nativeGesturesCheck, 0xf0, 0, 0) == 1
		return s
	}
	requestStop := func(exit bool) {
		if stop == nil {
			return
		}
		stopNotice = ""
		if settings.Mode != "extend" || o.PrepareStop == nil {
			closing = closing || exit
			close(stop)
			stop = nil
			update()
			setText(status, "Deteniendo y restaurando la pantalla…")
			return
		}
		token, start := stopGuard.begin(exit)
		if !start {
			return
		}
		update()
		go func() {
			err := o.PrepareStop()
			stopChecks <- stopCheckResult{token, err}
			call("PostMessageW", hwnd, stopPrepared, 0, 0)
		}()
	}
	quit := func() {
		if active && stop != nil {
			requestStop(true)
			return
		}
		if active || installing || disconnecting {
			closing = true
			update()
			return
		}
		call("DestroyWindow", hwnd)
	}
	show := func() {
		target := hwnd
		if testsOpen {
			target = call("FindWindowW", uintptr(unsafe.Pointer(p("RelevoNativeLaboratoryWindow"))), 0)
			if target == 0 {
				return
			}
		}
		call("ShowWindow", target, 9)
		call("SetForegroundWindow", target)
	}
	var command func(uintptr)
	command = func(id uintptr) {
		if testsOpen && id != 7 {
			return
		}
		if disconnecting && id != 6 && id != 7 && id != 10 {
			return
		}
		switch id {
		case 12:
			if o.OpenKeyboard != nil {
				if err := o.OpenKeyboard(); err != nil {
					setText(status, err.Error())
				}
			}
		case 1:
			if active || installing || disconnecting || o.Start == nil {
				return
			}
			settings = read()
			if e := SaveSettings(settings); e != nil {
				message(hwnd, e.Error())
				return
			}
			active = true
			stopGuard.newSession()
			stopNotice = ""
			stop = make(chan struct{})
			ch := stop
			s := settings
			update()
			go func() {
				report := func(text string) {
					mu.Lock()
					progressText = text
					mu.Unlock()
					call("PostMessageW", hwnd, progress, 0, 0)
				}
				var e error
				if s.AutoStart {
					e = autoConnect(ch, report, func() error { return o.Start(s, ch, report) }, 2*time.Second)
				} else {
					e = o.Start(s, ch, report)
				}
				mu.Lock()
				result = e
				mu.Unlock()
				call("PostMessageW", hwnd, finished, 0, 0)
			}()
		case 2:
			if stop != nil {
				requestStop(false)
				return
			}
			if active || installing || disconnecting || o.DisconnectMonitor == nil {
				return
			}
			settings = read()
			if settings.Mode != "extend" {
				return
			}
			disconnecting = true
			update()
			go func() {
				e := o.DisconnectMonitor()
				mu.Lock()
				result = e
				mu.Unlock()
				call("PostMessageW", hwnd, disconnected, 0, 0)
			}()
		case 3:
			if active || installing || disconnecting {
				return
			}
			installing = true
			update()
			go func() {
				var e error
				if o.Install != nil {
					e = o.Install()
				} else {
					e = InstallElevated()
				}
				mu.Lock()
				result = e
				mu.Unlock()
				call("PostMessageW", hwnd, installed, 0, 0)
			}()
		case 4:
			if e := Open("ms-settings:display"); e != nil {
				message(hwnd, e.Error())
			}
		case 5:
			message(hwnd, "USB requiere Apple Mobile Device Service. Conserva la instalación de Apple que ya funciona con tu iPad. Abre la app compatible en el iPad y acepta Confiar si se solicita. Se abrirá la guía oficial de Apple; no se instala nada automáticamente.")
			if e := Open("https://support.apple.com/guide/devices-windows/welcome/windows"); e != nil {
				message(hwnd, e.Error())
			}
		case 6:
			quit()
		case 7:
			show()
		case 9:
			if active || installing || disconnecting || o.OpenTests == nil {
				return
			}
			settings = read()
			if e := SaveSettings(settings); e != nil {
				message(hwnd, e.Error())
				return
			}
			testsOpen = true
			call("ShowWindow", hwnd, 0)
			notify(2)
			o.OpenTests()
			testsOpen = false
			notify(0)
			show()
		case 11:
			if active || installing || disconnecting || o.OpenYamTests == nil {
				return
			}
			settings = read()
			if e := SaveSettings(settings); e != nil {
				message(hwnd, e.Error())
				return
			}
			testsOpen = true
			call("ShowWindow", hwnd, 0)
			notify(2)
			o.OpenYamTests(settings)
			testsOpen = false
			notify(0)
			show()
		case 10:
			if o.ExportDiagnostic != nil {
				if e := o.ExportDiagnostic(); e != nil {
					message(hwnd, e.Error())
				}
			}
		case 28, 29:
			if !active && !installing && !disconnecting {
				if call("SendMessageW", clientCombo, 0x147, 0, 0) == 0 {
					call("SendMessageW", transportCombo, 0x14e, 0, 0)
				} else {
					call("SendMessageW", resCombo, 0x14e, 0, 0)
				}
				settings = read()
				update()
			}
		case 23:
			if !active && !installing && !disconnecting {
				settings = read()
				update()
			}
		case 32:
			old := settings.AutoStart
			candidate := read()
			if err := candidate.Validate(); err != nil {
				message(hwnd, err.Error())
				call("SendMessageW", autoStartCheck, 0xf1, boolValue(old), 0)
				return
			}
			if err := SetLoginStartup(candidate.AutoStart); err != nil {
				message(hwnd, err.Error())
				call("SendMessageW", autoStartCheck, 0xf1, boolValue(old), 0)
				return
			}
			if err := SaveSettings(candidate); err != nil {
				_ = SetLoginStartup(old)
				call("SendMessageW", autoStartCheck, 0xf1, boolValue(old), 0)
				message(hwnd, err.Error())
				return
			}
			settings = candidate
			setText(status, "Inicio automático guardado. Funciona después de iniciar sesión en Windows; conserva el ejecutable en esta carpeta.")
		case 8:
			if active || installing || disconnecting {
				return
			}
			settings = read()
			if e := SaveSettings(settings); e != nil {
				message(hwnd, e.Error())
			} else {
				update()
				setText(status, "Configuración guardada para el próximo inicio.")
			}
		}
	}
	taskbarCreated := uint32(call("RegisterWindowMessageW", uintptr(unsafe.Pointer(p("TaskbarCreated")))))
	wnd := syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
		if taskbarCreated != 0 && m == taskbarCreated {
			if testsOpen {
				return 0
			}
			notify(0)
			return 0
		}
		switch m {
		case 0x111:
			command(w & 0xffff)
			return 0
		case callback:
			if uint32(l) == 0x202 || uint32(l) == 0x203 {
				show()
				return 0
			}
			if uint32(l) == 0x205 {
				menu := call("CreatePopupMenu")
				defer call("DestroyMenu", menu)
				for _, item := range []struct {
					id   uintptr
					text string
				}{{7, "Abrir Relevo Display"}, {1, "Iniciar pantalla"}, {2, "Detener"}, {6, "Salir"}} {
					flags := uintptr(0)
					if item.id == 1 && (active || installing || disconnecting) || item.id == 2 && (installing || disconnecting || closing || stopGuard.pending || (active && stop == nil) || (!active && (settings.Mode != "extend" || o.DisconnectMonitor == nil))) {
						flags = 1
					}
					call("AppendMenuW", menu, flags, item.id, uintptr(unsafe.Pointer(p(item.text))))
				}
				var pt point
				call("GetCursorPos", uintptr(unsafe.Pointer(&pt)))
				call("SetForegroundWindow", h)
				id := call("TrackPopupMenu", menu, 0x102, uintptr(pt.X), uintptr(pt.Y), 0, h, 0)
				call("PostMessageW", h, 0, 0, 0)
				command(id)
			}
			return 0
		case progress:
			mu.Lock()
			text := progressText
			mu.Unlock()
			if active && !stopGuard.pending && stopNotice == "" {
				setText(status, text)
			}
			return 0
		case stopPrepared:
			result := <-stopChecks
			exit, valid := stopGuard.complete(result.token)
			if !valid || !active || stop == nil {
				return 0
			}
			if result.err != nil {
				closing = false
				update()
				stopNotice = "Detener cancelado; la pantalla sigue activa.\r\n" + result.err.Error()
				setText(status, stopNotice)
				show()
				return 0
			}
			closing = closing || exit
			close(stop)
			stop = nil
			update()
			setText(status, "Deteniendo y restaurando la pantalla…")
			return 0
		case finished, installed, disconnected:
			mu.Lock()
			e := result
			mu.Unlock()
			if m == finished {
				closing = stopGuard.finishSession() || closing
				stopNotice = ""
				active = false
				stop = nil
			} else if m == installed {
				installing = false
			} else {
				disconnecting = false
			}
			update()
			if closing {
				quit()
			} else if e != nil {
				setText(status, "La sesión terminó con error. Puedes volver a iniciar, abrir pruebas o exportar diagnóstico.\r\n"+e.Error())
				show()
			} else if m == installed {
				message(h, "Controlador preparado. Reinicia Windows antes de iniciar la pantalla extendida.")
			} else if m == disconnected || m == finished && settings.Mode == "extend" {
				setText(status, "Monitor virtual desconectado.")
			}
			return 0
		case 0x10:
			call("ShowWindow", h, 0)
			return 0
		case 2:
			notify(2)
			call("PostQuitMessage", 0)
			return 0
		}
		return call("DefWindowProcW", h, uintptr(m), w, l)
	})
	name := p("RelevoDisplayNotificationWindow")
	c := wc{Proc: wnd, Instance: instance, Name: name, Icon: hi, SmallIcon: hi, Cursor: call("LoadCursorW", 0, 32512), Background: 16}
	c.Size = uint32(unsafe.Sizeof(c))
	if call("RegisterClassExW", uintptr(unsafe.Pointer(&c))) == 0 {
		return fmt.Errorf("registrar ventana falló")
	}
	defer call("UnregisterClassW", uintptr(unsafe.Pointer(name)), instance)
	hwnd = call("CreateWindowExW", 0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p(o.Title))), 0x00ca0000, 0x80000000, 0x80000000, 560, 779, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("crear ventana falló")
	}
	font := syscall.NewLazyDLL("gdi32.dll")
	hf, _, _ := font.NewProc("GetStockObject").Call(17)
	childAt := func(class, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		ch := call("CreateWindowExW", 0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwnd, id, instance, 0)
		call("SendMessageW", ch, 0x30, hf, 1)
		return ch
	}
	child := func(class, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		if y >= 315 {
			y += 74
		}
		return childAt(class, text, style, x, y, w, h, id)
	}
	child("STATIC", "Tu iPad 2 como pantalla de Windows", 0, 24, 20, 500, 28, 0)
	child("STATIC", "Elige cliente y conexión · Abre la app del iPad · Inicia pantalla", 0, 24, 53, 500, 28, 0)
	child("STATIC", "Resolución (4:3)", 0, 24, 99, 180, 22, 0)
	child("STATIC", "Fluidez (FPS)", 0, 220, 99, 130, 22, 0)
	child("STATIC", "Calidad JPEG", 0, 370, 99, 140, 22, 0)
	resCombo = child("COMBOBOX", "", 0x200003|0x10000, 24, 124, 180, 180, 20)
	fpsCombo = child("COMBOBOX", "", 0x200003|0x10000, 220, 124, 130, 180, 21)
	qualityCombo = child("COMBOBOX", "", 0x200003|0x10000, 370, 124, 140, 180, 22)
	fill := func(h uintptr, values []string, index int) {
		for _, s := range values {
			call("SendMessageW", h, 0x143, 0, uintptr(unsafe.Pointer(p(s))))
		}
		call("SendMessageW", h, 0x14e, uintptr(index), 0)
	}
	ri, fi, qi := 0, 2, 1
	for i, r := range Resolutions {
		if r == settings.Resolution {
			ri = i
		}
	}
	for i, f := range FrameRates {
		if f == settings.FPS {
			fi = i
		}
	}
	for i, q := range JPEGQualities {
		if q == settings.Quality {
			qi = i
		}
	}
	fill(resCombo, Resolutions, ri)
	fill(fpsCombo, []string{"10", "15", "20", "30", "60"}, fi)
	fill(qualityCombo, []string{"60 · más fluido", "70 · equilibrado", "80 · más nítido", "90 · alta", "95 · muy alta", "100 · máxima"}, qi)
	child("STATIC", "Modo de pantalla", 0, 24, 168, 230, 22, 0)
	child("STATIC", "Posición al extender", 0, 280, 168, 230, 22, 0)
	modeCombo = child("COMBOBOX", "", 0x200003|0x10000, 24, 193, 230, 150, 23)
	positionCombo = child("COMBOBOX", "", 0x200003|0x10000, 280, 193, 230, 150, 24)
	mi, pi := 0, 0
	if settings.Mode == "desktop" {
		mi = 1
	}
	for i, p := range []string{"right", "left", "above", "below"} {
		if settings.Position == p {
			pi = i
		}
	}
	fill(modeCombo, []string{"Extender escritorio", "Ver pantalla principal"}, mi)
	fill(positionCombo, []string{"Derecha", "Izquierda", "Arriba", "Abajo"}, pi)
	awakeCheck = child("BUTTON", "Mantener pantalla encendida durante la sesión", 0x10003, 24, 230, 486, 26, 25)
	if settings.KeepAwake {
		call("SendMessageW", awakeCheck, 0xf1, 1, 0)
	}
	saveButton = child("BUTTON", "Guardar ajustes", 0x10000, 24, 264, 180, 32, 8)
	touchCheck = child("BUTTON", "Touch/gestos", 0x10003, 220, 264, 130, 32, 26)
	keyboardCheck = child("BUTTON", "Teclado Windows", 0x10003, 370, 264, 140, 32, 27)
	if settings.Touch {
		call("SendMessageW", touchCheck, 0xf1, 1, 0)
	}
	if settings.Keyboard {
		call("SendMessageW", keyboardCheck, 0xf1, 1, 0)
	}

	childAt("STATIC", "Cliente del iPad", 0, 24, 305, 180, 20, 0)
	childAt("STATIC", "Conexión", 0, 220, 305, 130, 20, 0)
	childAt("STATIC", "IP de Windows", 0, 370, 305, 140, 20, 0)
	clientCombo = childAt("COMBOBOX", "", 0x210003, 24, 329, 180, 150, 28)
	transportCombo = childAt("COMBOBOX", "", 0x210003, 220, 329, 130, 150, 29)
	lanCombo = childAt("COMBOBOX", "", 0x210003, 370, 329, 140, 150, 30)
	ci, ti, li := 0, 0, 0
	if settings.Receiver() == "relevo" {
		ci = 1
	}
	if settings.Link() == "WiFi" {
		ti = 1
	}
	for i, ip := range lanAddresses {
		if ip == settings.LANAddress {
			li = i
		}
	}
	fill(clientCombo, []string{"Yam (actual)", "Relevo (app propia)"}, ci)
	fill(transportCombo, []string{"USB", "Wi-Fi (QR)"}, ti)
	fill(lanCombo, lanAddresses, li)
	nativeGesturesCheck = childAt("BUTTON", "Gestos de Windows (dos dedos)", 0x10003, 24, 356, 486, 26, 31)
	if settings.NativeGestures {
		call("SendMessageW", nativeGesturesCheck, 0xf1, 1, 0)
	}
	startButton = child("BUTTON", "Iniciar pantalla", 0x10001, 24, 315, 180, 38, 1)
	stopButton = child("BUTTON", "Detener", 0x10000, 220, 315, 130, 38, 2)
	status = child("EDIT", "", 0x200000|0x4|0x40|0x800, 24, 371, 486, 42, 0) // Scrollable, read-only error/progress text; never a modal session error.
	installButton = child("BUTTON", "Preparar controlador", 0x10000, 24, 425, 180, 32, 3)
	child("BUTTON", "Pantallas de Windows", 0x10000, 220, 425, 160, 32, 4)
	child("BUTTON", "Ayuda USB", 0x10000, 392, 425, 118, 32, 5)
	testsButton = child("BUTTON", "Pruebas y nuevas funciones", 0x10000, 24, 470, 270, 34, 9)
	child("BUTTON", "Exportar diagnóstico", 0x10000, 308, 470, 202, 34, 10)
	yamTestsButton = child("BUTTON", "Pruebas con Yam", 0x10000, 24, 514, 270, 34, 11)
	child("BUTTON", "Abrir teclado", 0x10000, 308, 514, 202, 34, 12)
	child("STATIC", "Cerrar la ventana mantiene el icono junto al reloj. Para cerrar: icono → Salir.", 0, 24, 562, 490, 32, 0)
	autoStartCheck = childAt("BUTTON", "Conectar al iniciar sesión en Windows (Yam USB)", 0x10003, 24, 674, 490, 26, 32)
	call("SendMessageW", autoStartCheck, 0xf1, boolValue(settings.AutoStart), 0)
	childAt("STATIC", "Requiere sesión iniciada; no muestra la pantalla de acceso.", 0, 24, 706, 490, 22, 0)
	icon = nid{H: hwnd, ID: 1, Flags: 7, Callback: callback, Icon: hi}
	icon.Size = uint32(unsafe.Sizeof(icon))
	copy(icon.Tip[:], syscall.StringToUTF16(o.Title))
	r, _, _ := shell.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&icon)))
	if r == 0 {
		call("DestroyWindow", hwnd)
		return fmt.Errorf("Windows no pudo crear el icono de notificaciones")
	}
	update()
	call("ShowWindow", hwnd, 1)
	call("UpdateWindow", hwnd)
	if loadErr != nil {
		message(hwnd, "No se pudo leer la configuración guardada. Se usan valores predeterminados.\n"+loadErr.Error())
	}
	if loadErr == nil && settings.AutoStart && IsStartupLaunch() {
		call("PostMessageW", hwnd, 0x111, 1, 0)
	}
	var m msg
	for {
		r, _, e := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) == -1 {
			return fmt.Errorf("interfaz: %v", e)
		}
		if r == 0 {
			break
		}
		if call("IsDialogMessageW", hwnd, uintptr(unsafe.Pointer(&m))) == 0 {
			call("TranslateMessage", uintptr(unsafe.Pointer(&m)))
			call("DispatchMessageW", uintptr(unsafe.Pointer(&m)))
		}
	}
	return nil
}

func boolValue(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}
