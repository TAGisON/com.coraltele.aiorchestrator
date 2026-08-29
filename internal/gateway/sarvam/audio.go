package sarvam

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ResamplePCM converts mono s16le PCM from srcRate to dstRate (lab-quality linear).
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

// DecodeWAVorPCM extracts mono s16le PCM from a WAV container or returns raw bytes if not RIFF.
func DecodeWAVorPCM(data []byte) ([]byte, int, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return append([]byte(nil), data...), 0, nil
	}
	offset := 12
	var sampleRate int
	var bitsPerSample int
	var channels int
	var pcm []byte
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if offset+chunkSize > len(data) {
			break
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, 0, fmt.Errorf("wav fmt too short")
			}
			audioFormat := binary.LittleEndian.Uint16(data[offset:])
			channels = int(binary.LittleEndian.Uint16(data[offset+2:]))
			sampleRate = int(binary.LittleEndian.Uint32(data[offset+4:]))
			bitsPerSample = int(binary.LittleEndian.Uint16(data[offset+14:]))
			if audioFormat != 1 {
				return nil, 0, fmt.Errorf("unsupported wav format %d", audioFormat)
			}
		case "data":
			pcm = append([]byte(nil), data[offset:offset+chunkSize]...)
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if pcm == nil {
		return nil, 0, fmt.Errorf("wav missing data chunk")
	}
	if bitsPerSample != 16 {
		return nil, 0, fmt.Errorf("unsupported bits %d", bitsPerSample)
	}
	if channels == 2 {
		pcm = stereoToMono(pcm)
	} else if channels != 1 {
		return nil, 0, fmt.Errorf("unsupported channels %d", channels)
	}
	return pcm, sampleRate, nil
}

func stereoToMono(stereo []byte) []byte {
	n := len(stereo) / 4
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		l := int16(binary.LittleEndian.Uint16(stereo[i*4:]))
		r := int16(binary.LittleEndian.Uint16(stereo[i*4+2:]))
		v := int16((int32(l) + int32(r)) / 2)
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
