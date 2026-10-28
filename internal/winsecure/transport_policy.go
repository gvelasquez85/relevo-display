package winsecure

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// MIB_TCPROW_OWNER_PID has six DWORDs. Addresses/ports are stored in network
// byte order in their DWORD slots, while state, count and PID are native LE.
func acceptedSocketOwner(table []byte, serverIP net.IP, serverPort uint16, clientIP net.IP, clientPort uint16) (uint32, error) {
	const rowSize = 24
	if len(table) < 4 {
		return 0, errors.New("tabla de transporte USB incompleta")
	}
	count := binary.LittleEndian.Uint32(table[:4])
	if uint64(count) > uint64((len(table)-4)/rowSize) {
		return 0, errors.New("tabla de transporte USB inválida")
	}
	var owner uint32
	matches := 0
	for i := uint32(0); i < count; i++ {
		offset := 4 + int(i)*rowSize
		row := table[offset : offset+rowSize]
		if binary.LittleEndian.Uint32(row[:4]) != 5 || // MIB_TCP_STATE_ESTAB
			!net.IP(row[4:8]).Equal(serverIP) || binary.BigEndian.Uint16(row[8:10]) != serverPort ||
			!net.IP(row[12:16]).Equal(clientIP) || binary.BigEndian.Uint16(row[16:18]) != clientPort {
			continue
		}
		owner = binary.LittleEndian.Uint32(row[20:24])
		matches++
	}
	if matches != 1 || owner == 0 {
		return 0, fmt.Errorf("no se identificó una única conexión USB de Apple activa")
	}
	return owner, nil
}
