//go:build windows && amd64

package wintray

import "unsafe"

// These compile-time assertions run during cross-builds, without a Windows VM.
var (
	_ [976 - int(unsafe.Sizeof(nid{}))]byte
	_ [int(unsafe.Sizeof(nid{})) - 976]byte
	_ [80 - int(unsafe.Sizeof(wc{}))]byte
	_ [int(unsafe.Sizeof(wc{})) - 80]byte
	_ [48 - int(unsafe.Sizeof(msg{}))]byte
	_ [int(unsafe.Sizeof(msg{})) - 48]byte
	_ [112 - int(unsafe.Sizeof(shellExecuteInfo{}))]byte
	_ [int(unsafe.Sizeof(shellExecuteInfo{})) - 112]byte
)
