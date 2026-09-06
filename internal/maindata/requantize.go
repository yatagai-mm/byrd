package maindata

import (
	"fmt"
	"github.com/yatagai-mm/byrd/internal/common"
	"math"
)

const (
	mixedLongEndLine   = 36
	mixedShortStartSFB = 3
	minRequantizeQ     = -45 // global_gain = 255, no attenuation
	maxRequantizeQ     = 326 // global_gain = 0, subblock_gain = 7, scalefactor = 15
)

// MPEG-1 Huffman magnitudes are at most 15 + (1<<13) - 1. Build the
// lookup with the original formula so the float32 results stay identical.
var pow43Table = func() [8207]float32 {
	var table [8207]float32
	for i := range table {
		table[i] = float32(math.Pow(float64(i), 4.0/3.0))
	}
	return table
}()

var requantizeScaleTable = func() [maxRequantizeQ - minRequantizeQ + 1]float32 {
	var table [maxRequantizeQ - minRequantizeQ + 1]float32
	for i := range table {
		table[i] = float32(math.Pow(2, -float64(i+minRequantizeQ)/4))
	}
	return table
}()

func requantizeScale(q int) float32 {
	if q >= minRequantizeQ && q <= maxRequantizeQ {
		return requantizeScaleTable[q-minRequantizeQ]
	}
	// Keep the previous behavior for callers supplying values beyond the
	// bitstream field ranges, rather than indexing outside the table.
	return float32(math.Pow(2, -float64(q)/4))
}

func Requantize(sampleRate uint16, gc *common.GranuleChannelInfo, scalefactors *Scalefactors, spectralValues []int, out *[]float32) error {
	if gc == nil {
		return fmt.Errorf("nil granule channel info")
	}
	if scalefactors == nil {
		return fmt.Errorf("nil scalefactors")
	}
	if out == nil {
		return fmt.Errorf("nil output buffer")
	}
	if len(*out) != 576 {
		return fmt.Errorf("invalid output buffer length: %d", len(*out))
	}
	if len(spectralValues) != 576 {
		return fmt.Errorf("invalid spectral values length: %d", len(spectralValues))
	}
	sfBands, ok := common.SCALEFACTOR_BAND_INDICES[sampleRate]
	if !ok {
		return fmt.Errorf("unsupported sample rate for scalefactor bands: %d", sampleRate)
	}

	clear(*out)

	for i, is := range spectralValues {
		if is == 0 {
			continue
		}

		switch {
		case !gc.GetWindowSwitching() || gc.GetBlockType() != common.BlockTypeShort:
			sfb := longSFBForLine(sfBands.Long, i)
			(*out)[i] = requantizeLongLine(is, gc, scalefactors, sfb)
		case gc.GetMixedBlockFlag() && i < mixedLongEndLine:
			sfb := longSFBForLine(sfBands.Long, i)
			(*out)[i] = requantizeLongLine(is, gc, scalefactors, sfb)
		default:
			sfb, win, err := shortSFBWindowForLine(sfBands.Short, i, gc.GetMixedBlockFlag())
			if err != nil {
				return err
			}
			(*out)[i] = requantizeShortLine(is, gc, scalefactors, sfb, win)
		}
	}

	return nil
}

func requantizeLongLine(is int, gc *common.GranuleChannelInfo, scalefactors *Scalefactors, sfb int) float32 {
	pretab := 0
	if gc.GetPreflag() && sfb < len(common.PRETAB) {
		pretab = int(common.PRETAB[sfb])
	}
	scalefacMultiplier := 1
	if gc.GetScalefacScale() {
		scalefacMultiplier = 2
	}

	// xr[i] = sign(is[i]) * |is[i]|^(4/3) *
	//   2^(-(210 - global_gain + 2*(1+scalefac_scale)*(scalefac_l[sfb] + preflag*pretab[sfb])) / 4)
	q := 210 - int(gc.GlobalGain) + 2*scalefacMultiplier*(int(scalefactors.Long[sfb])+pretab)
	return signedPow43(is) * requantizeScale(q)
}

func requantizeShortLine(is int, gc *common.GranuleChannelInfo, scalefactors *Scalefactors, sfb int, win int) float32 {
	scalefacMultiplier := 1
	if gc.GetScalefacScale() {
		scalefacMultiplier = 2
	}

	// xr[i] = sign(is[i]) * |is[i]|^(4/3) *
	//   2^(-(210 - global_gain + 8*subblock_gain[w] + 2*(1+scalefac_scale)*scalefac_s[sfb][w]) / 4)
	q := 210 - int(gc.GlobalGain) + 8*int(gc.SubblockGain[win]) + 2*scalefacMultiplier*int(scalefactors.Short[sfb][win])
	return signedPow43(is) * requantizeScale(q)
}

func signedPow43(is int) float32 {
	if is >= 0 && is < len(pow43Table) {
		return pow43Table[is]
	}
	if is < 0 && is > -len(pow43Table) {
		return -pow43Table[-is]
	}
	mag := float32(math.Pow(math.Abs(float64(is)), 4.0/3.0))
	return float32(math.Copysign(float64(mag), float64(is)))
}

func longSFBForLine(longBands [23]int, line int) int {
	for sfb := 0; sfb < len(longBands)-2; sfb++ {
		if line < longBands[sfb+1] {
			return sfb
		}
	}
	return len(longBands) - 3
}

func shortSFBWindowForLine(shortBands [14]int, line int, mixed bool) (int, int, error) {
	shortLine := line
	startSFB := 0
	if mixed {
		if line < mixedLongEndLine {
			return 0, 0, fmt.Errorf("line %d is in mixed long-block region", line)
		}
		shortLine = line - mixedLongEndLine + shortBands[mixedShortStartSFB]*SCALEFACTOR_SHORT_WINDOW_COUNT
		startSFB = mixedShortStartSFB
	}

	for sfb := startSFB; sfb < len(shortBands)-2; sfb++ {
		bandStart := shortBands[sfb] * SCALEFACTOR_SHORT_WINDOW_COUNT
		bandEnd := shortBands[sfb+1] * SCALEFACTOR_SHORT_WINDOW_COUNT
		if shortLine >= bandStart && shortLine < bandEnd {
			width := shortBands[sfb+1] - shortBands[sfb]
			if width <= 0 {
				return 0, 0, fmt.Errorf("invalid short scalefactor band width at sfb %d", sfb)
			}
			window := (shortLine - bandStart) / width
			if window < 0 || window >= SCALEFACTOR_SHORT_WINDOW_COUNT {
				return 0, 0, fmt.Errorf("invalid short window index for line %d", line)
			}
			return sfb, window, nil
		}
	}

	// The current scalefactor parser stores 12 short scalefactor bands. Until the
	// high-band handling is expanded, treat the remaining tail as part of the
	// last transmitted short scalefactor band.
	lastSFB := len(shortBands) - 3
	bandStart := shortBands[lastSFB] * SCALEFACTOR_SHORT_WINDOW_COUNT
	bandEnd := shortBands[len(shortBands)-1] * SCALEFACTOR_SHORT_WINDOW_COUNT
	if shortLine >= bandStart && shortLine < bandEnd {
		width := shortBands[len(shortBands)-1] - shortBands[lastSFB]
		if width <= 0 {
			return 0, 0, fmt.Errorf("invalid terminal short scalefactor band width")
		}
		window := (shortLine - bandStart) / width
		if window < 0 || window >= SCALEFACTOR_SHORT_WINDOW_COUNT {
			return 0, 0, fmt.Errorf("invalid terminal short window index for line %d", line)
		}
		return lastSFB, window, nil
	}

	return 0, 0, fmt.Errorf("line %d does not map to a short scalefactor band", line)
}
