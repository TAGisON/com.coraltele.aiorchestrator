// Package modaudiostream is the FreeSWITCH mod_audio_stream edge (EDGE_FS.md).
// FS JSON/binary dialect never leaves this package.
package modaudiostream

import "github.com/coraltele/com.coraltele.aiorchestrator/internal/audio"

// ResamplePCM converts mono s16le PCM from srcRate to dstRate.
func ResamplePCM(src []byte, srcRate, dstRate int) []byte {
	return audio.Resample(src, srcRate, dstRate)
}

// FrameBytes returns mono s16le bytes for one frame at rateHz and frameMs.
func FrameBytes(rateHz, frameMs int) int {
	return audio.FrameBytes(rateHz, frameMs)
}
