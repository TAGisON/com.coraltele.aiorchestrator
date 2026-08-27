// Package modaudiostream is the FreeSWITCH mod_audio_stream edge (EDGE_FS.md).
// FS JSON/binary dialect never leaves this package.
package modaudiostream

import (
	"encoding/binary"
	"math"
)

// ResamplePCM converts mono s16le PCM from srcRate to dstRate using linear interpolation (lab-quality).
func ResamplePCM(src []byte, srcRate, dstRate int) []byte {
	if srcRate <= 0 || dstRate <= 0 || len(src) < 2 {
		return append([]byte(nil), src...)
	}
	if srcRate == dstRate {
		out := make([]byte, len(src))
		copy(out, src)
		return out
	}
	srcN := len(src) / 2
	if srcN == 0 {
		return nil
	}
	dstN := int(math.Round(float64(srcN) * float64(dstRate) / float64(srcRate)))
	if dstN < 1 {
		dstN = 1
	}
	out := make([]byte, dstN*2)
	for i := 0; i < dstN; i++ {
		pos := float64(i) * float64(srcRate) / float64(dstRate)
		i0 := int(pos)
		if i0 >= srcN-1 {
			i0 = srcN - 1
			s := int16(binary.LittleEndian.Uint16(src[i0*2:]))
			binary.LittleEndian.PutUint16(out[i*2:], uint16(s))
			continue
		}
		frac := pos - float64(i0)
		s0 := float64(int16(binary.LittleEndian.Uint16(src[i0*2:])))
		s1 := float64(int16(binary.LittleEndian.Uint16(src[(i0+1)*2:])))
		v := int16(math.Round(s0 + (s1-s0)*frac))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}

// FrameBytes returns mono s16le bytes for one frame at rateHz and frameMs.
func FrameBytes(rateHz, frameMs int) int {
	if rateHz <= 0 {
		rateHz = 8000
	}
	if frameMs <= 0 {
		frameMs = 20
	}
	return rateHz * 2 * frameMs / 1000
}
