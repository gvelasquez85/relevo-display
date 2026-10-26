package desktop

// bgraToRGBA copies an opaque GDI frame without changing its native backing
// memory; subsequent drawing and native encoders still receive BGRA.
func bgraToRGBA(dst, src []byte) {
	for i := 0; i+3 < len(src); i += 4 {
		dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], 255
	}
}
