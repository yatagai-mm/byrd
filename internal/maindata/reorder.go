package maindata

import (
	"fmt"
	"github.com/yatagai-mm/byrd/internal/common"
)

func Reorder(sampleRate uint16, gc *common.GranuleChannelInfo, in []float32, out *[]float32) error {
	if gc == nil {
		return fmt.Errorf("nil granule channel info")
	}
	if out == nil {
		return fmt.Errorf("nil output buffer")
	}
	if len(in) != 576 {
		return fmt.Errorf("invalid input buffer length: %d", len(in))
	}
	if len(*out) != 576 {
		return fmt.Errorf("invalid output buffer length: %d", len(*out))
	}

	// Reordering applies only to short blocks. Long/start/end blocks stay in place.
	if !gc.GetWindowSwitching() || gc.GetBlockType() != common.BlockTypeShort {
		copy(*out, in)
		return nil
	}

	sfBands, ok := common.SCALEFACTOR_BAND_INDICES[sampleRate]
	if !ok {
		return fmt.Errorf("unsupported sample rate for scalefactor bands: %d", sampleRate)
	}

	clear(*out)
	startSFB := 0
	srcPos := 0
	if gc.GetMixedBlockFlag() {
		copy((*out)[:mixedLongEndLine], in[:mixedLongEndLine])
		startSFB = mixedShortStartSFB
		srcPos = mixedLongEndLine
	}

	for sfb := startSFB; sfb < len(sfBands.Short)-1; sfb++ {
		width := sfBands.Short[sfb+1] - sfBands.Short[sfb]
		if width <= 0 {
			return fmt.Errorf("invalid short scalefactor band width at sfb %d", sfb)
		}
		dstBase := sfBands.Short[sfb] * SCALEFACTOR_SHORT_WINDOW_COUNT
		for win := 0; win < SCALEFACTOR_SHORT_WINDOW_COUNT; win++ {
			for freq := 0; freq < width; freq++ {
				if srcPos >= len(in) {
					return fmt.Errorf("short block reorder exceeded input at sfb %d", sfb)
				}
				dst := dstBase + (freq * SCALEFACTOR_SHORT_WINDOW_COUNT) + win
				if dst >= len(*out) {
					return fmt.Errorf("short block reorder exceeded output at sfb %d", sfb)
				}
				(*out)[dst] = in[srcPos]
				srcPos++
			}
		}
	}

	if srcPos != len(in) {
		return fmt.Errorf("short block reorder consumed %d lines, want %d", srcPos, len(in))
	}
	return nil
}
