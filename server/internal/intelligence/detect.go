package intelligence

import (
	"encoding/binary"
	"fmt"
	"math"
)

// DetectRecurring finds an offset-tolerant contiguous run shared by at least
// three timelines. Each signature represents a two-second bounded audio chunk.
func DetectRecurring(series [][]string) (start, end, score float64, ok bool) {
	if len(series) < 3 {
		return
	}
	anchor := series[0]
	bestStart, bestLen, support := 0, 0, 1
	for i := range anchor {
		for _, other := range series[1:] {
			for j := range other {
				length := 0
				for i+length < len(anchor) && j+length < len(other) && signatureEqual(anchor[i+length], other[j+length]) {
					length++
				}
				if length > bestLen {
					bestStart, bestLen, support = i, length, 1
				}
			}
		}
	}
	if bestLen < 5 {
		return
	}
	for _, other := range series[1:] {
		found := false
		for j := 0; j+bestLen <= len(other); j++ {
			found = true
			for k := 0; k < bestLen; k++ {
				if !signatureEqual(anchor[bestStart+k], other[j+k]) {
					found = false
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			support++
		}
	}
	if support < 3 {
		return
	}
	score = math.Min(.99, .55+.08*float64(bestLen)+.05*float64(support-3))
	return float64(bestStart * 2), float64((bestStart + bestLen) * 2), score, true
}

func signatureEqual(a, b string) bool {
	if a == b {
		return true
	}
	var az, ae, bz, be int
	if _, e := fmt.Sscanf(a, "z%d:e%d", &az, &ae); e != nil {
		return false
	}
	if _, e := fmt.Sscanf(b, "z%d:e%d", &bz, &be); e != nil {
		return false
	}
	return abs(az-bz) <= 1 && abs(ae-be) <= 1
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func MatchStart(series, pattern []string) (int, bool) {
	for i := 0; i+len(pattern) <= len(series); i++ {
		ok := true
		for j := range pattern {
			if !signatureEqual(series[i+j], pattern[j]) {
				ok = false
				break
			}
		}
		if ok {
			return i, true
		}
	}
	return 0, false
}

func ChunkSignatures(pcm []byte, bytesPerChunk int) []string {
	if bytesPerChunk < 1 {
		return nil
	}
	out := []string{}
	for i := 0; i+bytesPerChunk <= len(pcm); i += bytesPerChunk {
		chunk := pcm[i : i+bytesPerChunk]
		crossings, energy := 0, int64(0)
		var previous int16
		for j := 0; j+1 < len(chunk); j += 2 {
			sample := int16(binary.LittleEndian.Uint16(chunk[j : j+2]))
			if j > 0 && (sample < 0) != (previous < 0) {
				crossings++
			}
			v := int64(sample)
			if v < 0 {
				v = -v
			}
			energy += v
			previous = sample
		}
		// Coarse quantization tolerates codec/resampler noise without pretending
		// that weakly similar audio is an exact fingerprint match.
		out = append(out, fmt.Sprintf("z%d:e%d", crossings/20, (energy/int64(bytesPerChunk/2))/250))
	}
	return out
}

// DetectCredits returns a conservative sustained low-luma tail. Means are
// sparse one-frame-per-second samples; it does not equate the last 10% with credits.
func DetectCredits(means []float64, duration float64) (start, end, score float64, ok bool) {
	if len(means) < 12 || duration <= 0 {
		return
	}
	runStart := -1
	for i, v := range means {
		if v < 38 {
			if runStart < 0 {
				runStart = i
			}
		} else if i-runStart < 8 {
			runStart = -1
		}
	}
	if runStart < 0 || len(means)-runStart < 10 {
		return
	}
	start = duration - float64(len(means)-runStart)
	if start < 0 {
		start = 0
	}
	end = duration
	score = math.Min(.94, .72+float64(len(means)-runStart)*.012)
	return start, end, score, true
}
