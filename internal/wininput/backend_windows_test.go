package wininput

import (
	"testing"
	"unsafe"
)

func TestNativeInputABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Windows x64 ABI")
	}
	var input struct {
		Kind              uint32
		Pad               uint32
		X, Y              int32
		Data, Flags, Time uint32
		Pad2              uint32
		Extra             uintptr
	}
	if unsafe.Sizeof(input) != 40 || unsafe.Offsetof(input.X) != 8 || unsafe.Offsetof(input.Flags) != 20 || unsafe.Offsetof(input.Extra) != 32 {
		t.Fatal("incorrect SendInput ABI")
	}
}
