// Package audio holds PCM helpers shared by the edges, gateways and the
// fallback prompt store: rate conversion, frame sizing and WAV decoding.
//
// Canonical PCM everywhere in the orchestrator is mono signed 16-bit
// little-endian; anything entering the system is normalised to that here.
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// MinRateHz and MaxRateHz bound the sample rates we accept from any source.
const (
	MinRateHz = 8000
	MaxRateHz = 48000
)

// Resample converts mono s16le PCM from srcRate to dstRate using linear
// interpolation. It is intentionally cheap: telephony audio is band-limited to
// 3.4 kHz, so the artefacts are below the codec noise floor.
func Resample(src []byte, srcRate, dstRate int) []byte {
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

// DownmixToMono averages interleaved channels into mono s16le. Input with one
// channel is returned unchanged.
func DownmixToMono(pcm []byte, channels int) []byte {
	if channels <= 1 {
		return append([]byte(nil), pcm...)
	}
	frame := channels * 2
	n := len(pcm) / frame
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		sum := 0
		base := i * frame
		for c := 0; c < channels; c++ {
			sum += int(int16(binary.LittleEndian.Uint16(pcm[base+c*2:])))
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(sum/channels)))
	}
	return out
}

// ErrNotWAV is returned when the payload is not a RIFF/WAVE container.
var ErrNotWAV = errors.New("audio: not a RIFF/WAVE file")

// WAV is a decoded, normalised WAVE file: mono s16le at SampleRate.
type WAV struct {
	PCM        []byte
	SampleRate int
	// SourceChannels is what the file declared before downmixing.
	SourceChannels int
	// DurationMs is derived from the normalised PCM.
	DurationMs int
}

// DecodeWAV parses a PCM WAVE file and normalises it to mono s16le.
//
// It is deliberately strict — this is the path uploaded operator prompts take,
// and a prompt that fails to decode at upload time is far better than one that
// fails while a caller is on the line. Only uncompressed 16-bit PCM is accepted.
func DecodeWAV(data []byte) (WAV, error) {
	var out WAV
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return out, ErrNotWAV
	}

	var (
		sampleRate    int
		bitsPerSample int
		channels      int
		format        uint16
		pcm           []byte
		sawFmt        bool
	)

	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(data) {
			// Truncated final chunk: take what is actually there for data,
			// otherwise stop.
			if chunkID == "data" && offset < len(data) {
				chunkSize = len(data) - offset
			} else {
				break
			}
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return out, errors.New("audio: wav fmt chunk too short")
			}
			format = binary.LittleEndian.Uint16(data[offset:])
			channels = int(binary.LittleEndian.Uint16(data[offset+2:]))
			sampleRate = int(binary.LittleEndian.Uint32(data[offset+4:]))
			bitsPerSample = int(binary.LittleEndian.Uint16(data[offset+14:]))
			sawFmt = true
		case "data":
			pcm = data[offset : offset+chunkSize]
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++ // chunks are word aligned
		}
	}

	if !sawFmt {
		return out, errors.New("audio: wav has no fmt chunk")
	}
	// 0xFFFE is WAVE_FORMAT_EXTENSIBLE, which is PCM when the sub-format says so.
	// We accept it only for 16-bit, which is the case we actually care about.
	if format != 1 && !(format == 0xFFFE && bitsPerSample == 16) {
		return out, fmt.Errorf("audio: unsupported wav format %d (need uncompressed PCM)", format)
	}
	if bitsPerSample != 16 {
		return out, fmt.Errorf("audio: unsupported bit depth %d (need 16)", bitsPerSample)
	}
	if channels < 1 || channels > 2 {
		return out, fmt.Errorf("audio: unsupported channel count %d (need 1 or 2)", channels)
	}
	if sampleRate < MinRateHz || sampleRate > MaxRateHz {
		return out, fmt.Errorf("audio: sample rate %d out of range %d–%d", sampleRate, MinRateHz, MaxRateHz)
	}
	if len(pcm) == 0 {
		return out, errors.New("audio: wav has no audio data")
	}

	mono := DownmixToMono(pcm, channels)
	// Guard against a truncated final sample.
	if len(mono)%2 == 1 {
		mono = mono[:len(mono)-1]
	}
	if len(mono) == 0 {
		return out, errors.New("audio: wav decoded to zero samples")
	}

	out.PCM = mono
	out.SampleRate = sampleRate
	out.SourceChannels = channels
	out.DurationMs = len(mono) / 2 * 1000 / sampleRate
	return out, nil
}

// EncodeWAV wraps mono s16le PCM in a canonical 44-byte PCM WAVE header.
func EncodeWAV(pcm []byte, sampleRate int) []byte {
	const channels, bytesPerSample = 1, 2
	blockAlign := channels * bytesPerSample
	byteRate := sampleRate * blockAlign

	out := make([]byte, 44+len(pcm))
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+len(pcm)))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], channels)
	binary.LittleEndian.PutUint32(out[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(out[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:36], uint16(bytesPerSample*8))
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}
