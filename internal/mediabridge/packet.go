// Package mediabridge encodes the local media-provider protocol. It does not
// create a pipe, grant access, or expose a network listener. Transport activation
// requires separate Windows identity/ACL validation.
package mediabridge

import (
	"encoding/binary"
	"fmt"
)

type Kind uint16

const (
	CameraJPEG    Kind = 1
	MicrophonePCM Kind = 2
	Disconnect    Kind = 3
)

// Encode copies a validated RVL1 media payload into an RVM1 packet. timestampUS
// must be host monotonic time, not the iPad clock. Sequence is informational.
// Full JPEG decoding and dimension checks belong to the camera provider.
func Encode(kind Kind, sequence uint32, timestampUS uint64, payload []byte) ([]byte, error) {
	n := len(payload)
	valid := false
	switch kind {
	case CameraJPEG:
		valid = n >= 4 && n <= 512*1024 && payload[0] == 255 && payload[1] == 216 && payload[n-2] == 255 && payload[n-1] == 217
	case MicrophonePCM:
		valid = n >= 10 && n <= 890 && n%2 == 0 && binary.LittleEndian.Uint32(payload) == 22050 && binary.LittleEndian.Uint16(payload[4:]) == 1 && binary.LittleEndian.Uint16(payload[6:]) == 16
	case Disconnect:
		valid = n == 0
	}
	if !valid {
		return nil, fmt.Errorf("invalid media provider payload for kind %d", kind)
	}
	b := make([]byte, 24+n)
	copy(b, "RVM1")
	binary.LittleEndian.PutUint16(b[4:], 1)
	binary.LittleEndian.PutUint16(b[6:], uint16(kind))
	binary.LittleEndian.PutUint32(b[8:], sequence)
	binary.LittleEndian.PutUint32(b[12:], uint32(n))
	binary.LittleEndian.PutUint64(b[16:], timestampUS)
	copy(b[24:], payload)
	return b, nil
}
