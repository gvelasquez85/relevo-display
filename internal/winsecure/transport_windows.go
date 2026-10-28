//go:build windows

package winsecure

import (
	"errors"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getExtendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

// VerifyAppleUSBConnection proves that this exact established loopback socket
// terminates in the running Apple Mobile Device Service process. Merely finding
// a service or a listener on port 27015 would allow a local user's fake usbmux
// server to feed privileged sign-in input. Call before sending protocol bytes.
func VerifyAppleUSBConnection(conn net.Conn) error {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("el broker requiere el transporte TCP USB local de Apple")
	}
	local, lok := tcp.LocalAddr().(*net.TCPAddr)
	remote, rok := tcp.RemoteAddr().(*net.TCPAddr)
	if !lok || !rok || local.Port < 1 || local.Port > 65535 || remote.Port != 27015 ||
		local.IP.To4() == nil || remote.IP.To4() == nil ||
		!local.IP.Equal(net.IPv4(127, 0, 0, 1)) || !remote.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		return errors.New("el broker requiere USB de Apple en loopback IPv4")
	}
	name, _ := windows.UTF16PtrFromString("Apple Mobile Device Service")
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return errors.New("no se pudo verificar el servicio USB de Apple")
	}
	defer windows.CloseServiceHandle(manager)
	service, err := windows.OpenService(manager, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return errors.New("no está disponible el servicio USB de Apple reconocido")
	}
	defer windows.CloseServiceHandle(service)
	before, err := appleServicePID(service)
	if err != nil {
		return err
	}
	table, err := tcpOwners()
	if err != nil {
		return err
	}
	owner, err := acceptedSocketOwner(table, remote.IP.To4(), uint16(remote.Port), local.IP.To4(), uint16(local.Port))
	if err != nil {
		return err
	}
	after, err := appleServicePID(service)
	if err != nil {
		return err
	}
	if owner == 0 || owner != before || before != after {
		return errors.New("la conexión USB local no pertenece al servicio de Apple")
	}
	return nil
}
func appleServicePID(service windows.Handle) (uint32, error) {
	var status windows.SERVICE_STATUS_PROCESS
	var needed uint32
	if windows.QueryServiceStatusEx(service, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&status)), uint32(unsafe.Sizeof(status)), &needed) != nil ||
		status.CurrentState != windows.SERVICE_RUNNING || status.ProcessId == 0 ||
		status.ServiceType&windows.SERVICE_WIN32_OWN_PROCESS == 0 {
		return 0, errors.New("el servicio USB de Apple no está ejecutándose como proceso propio")
	}
	return status.ProcessId, nil
}
func tcpOwners() ([]byte, error) {
	var size uint32
	result, _, _ := getExtendedTCPTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 4, 0) // TCP_TABLE_OWNER_PID_CONNECTIONS
	if result != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, errors.New("no se pudo consultar el propietario del transporte USB")
	}
	for attempt := 0; attempt < 3; attempt++ {
		if size < 4 || size > 4*1024*1024 {
			return nil, errors.New("tabla de transporte USB fuera de límites")
		}
		data := make([]byte, size)
		result, _, _ = getExtendedTCPTable.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 4, 0)
		if result == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			continue
		}
		if result != 0 || size > uint32(len(data)) {
			return nil, errors.New("no se pudo verificar la conexión USB activa")
		}
		return data[:size], nil
	}
	return nil, errors.New("el transporte USB cambió durante su verificación")
}
