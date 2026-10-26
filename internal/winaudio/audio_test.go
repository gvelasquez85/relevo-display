package winaudio

import (
	"encoding/binary"
	"relevo/internal/nativewire"
	"testing"
)

func TestResampleStreamingPackets(t *testing.T) {
	r := resampler{rate: 48000}
	total := 0
	for k := 0; k < 100; k++ {
		in := make([]float64, 480)
		for i := range in {
			in[i] = .5
		}
		for _, p := range r.feed(in) {
			if e := nativewire.ValidatePCM(p); e != nil {
				t.Fatal(e)
			}
			if v := int16(binary.LittleEndian.Uint16(p[8:])); v < 16000 || v > 17000 {
				t.Fatal(v)
			}
			total++
		}
	}
	if total < 49 || total > 50 {
		t.Fatal(total)
	}
}
func TestResamplerPacketBoundaryContinuity(t *testing.T) {
	a, b := resampler{rate: 44100}, resampler{rate: 44100}
	in := make([]float64, 4410)
	for i := range in {
		in[i] = float64(i%100) / 100
	}
	whole := a.feed(in)
	var split [][]byte
	for i := 0; i < len(in); i += 7 {
		end := i + 7
		if end > len(in) {
			end = len(in)
		}
		split = append(split, b.feed(in[i:end])...)
	}
	if len(whole) != len(split) {
		t.Fatal(len(whole), len(split))
	}
	for i := range whole {
		if string(whole[i]) != string(split[i]) {
			t.Fatal("packet discontinuity", i)
		}
	}
}
