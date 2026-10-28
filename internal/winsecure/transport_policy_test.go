package winsecure

import (
	"encoding/binary"
	"net"
	"testing"
)

func tcpRow(state, pid uint32, serverPort, clientPort uint16) []byte {
	b := make([]byte, 24)
	binary.LittleEndian.PutUint32(b, state)
	copy(b[4:8], []byte{127, 0, 0, 1})
	binary.BigEndian.PutUint16(b[8:10], serverPort)
	copy(b[12:16], []byte{127, 0, 0, 1})
	binary.BigEndian.PutUint16(b[16:18], clientPort)
	binary.LittleEndian.PutUint32(b[20:], pid)
	return b
}
func TestAcceptedSocketOwnerMatchesExactReverseTuple(t *testing.T) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, 3)
	b = append(b, tcpRow(5, 22, 27015, 49152)...)
	b = append(b, tcpRow(5, 33, 27015, 49153)...)
	b = append(b, tcpRow(5, 44, 49152, 27015)...)
	ip := net.IPv4(127, 0, 0, 1)
	pid, err := acceptedSocketOwner(b, ip, 27015, ip, 49152)
	if err != nil || pid != 22 {
		t.Fatal(pid, err)
	}
}
func TestAcceptedSocketOwnerRejectsMissingDuplicateAndMalformed(t *testing.T) {
	ip := net.IPv4(127, 0, 0, 1)
	table := func(rows ...[]byte) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(len(rows)))
		for _, r := range rows {
			b = append(b, r...)
		}
		return b
	}
	for _, b := range [][]byte{
		nil, {1, 0, 0, 0}, table(tcpRow(2, 1, 27015, 49152)), table(tcpRow(5, 0, 27015, 49152)),
		table(tcpRow(5, 1, 27015, 49152), tcpRow(5, 2, 27015, 49152)),
		table(tcpRow(5, 1, 27015, 49153)),
	} {
		if _, err := acceptedSocketOwner(b, ip, 27015, ip, 49152); err == nil {
			t.Fatal("accepted invalid ownership table")
		}
	}
}
