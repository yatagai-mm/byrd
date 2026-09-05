package stereo

import (
	"fmt"

	"github.com/yatagai-mm/byrd/internal/common"
	"github.com/yatagai-mm/byrd/internal/header"
	"github.com/yatagai-mm/byrd/internal/maindata"
)

const MS_STEREO_SCALE float32 = 0.70710677 // 1 / sqrt(2)

var intensityStereoRatios = [6]float32{
	0.000000,
	0.267949,
	0.577350,
	1.000000,
	1.732051,
	3.732051,
}

// MS Stereo records the mid (M) and side (S) signals instead of left and right.
// To reconstruct the left and right channels, we can use the following formulas:
// Left = (M + S) / sqrt(2)
// Right = (M - S) / sqrt(2)
func ApplyMSStereo(left []float32, right []float32) error {
	if len(left) != 576 || len(right) != 576 {
		return fmt.Errorf("ms stereo requires 576 spectral lines: left=%d right=%d", len(left), len(right))
	}

	for i := range left {
		m := left[i]
		s := right[i]
		left[i] = (m + s) * MS_STEREO_SCALE
		right[i] = (m - s) * MS_STEREO_SCALE
	}

	return nil
}

func ApplyJointStereo(sampleRate uint16, channelMode header.ChannelMode, modeExt header.ModeExtension, gc *common.GranuleChannelInfo, scalefactors *maindata.Scalefactors, left []float32, right []float32, leftCount1 int, rightCount1 int) error {
	if channelMode != header.ChannelModeJointStereo {
		return nil
	}
	if len(left) != 576 || len(right) != 576 {
		return fmt.Errorf("joint stereo requires 576 spectral lines: left=%d right=%d", len(left), len(right))
	}
	maxPos := leftCount1
	if rightCount1 > maxPos {
		maxPos = rightCount1
	}
	if modeExt == header.ModeExtensionMSStereo || modeExt == header.ModeExtensionIntensityAndMS {
		if err := applyMSStereoUpTo(left, right, maxPos); err != nil {
			return err
		}
	}
	if modeExt != header.ModeExtensionIntensityStereo && modeExt != header.ModeExtensionIntensityAndMS {
		return nil
	}
	if gc == nil {
		return fmt.Errorf("nil granule channel info")
	}
	if scalefactors == nil {
		return fmt.Errorf("nil scalefactors")
	}

	return applyIntensityStereo(sampleRate, gc, scalefactors, rightCount1, left, right)
}

func applyMSStereoUpTo(left []float32, right []float32, maxPos int) error {
	if len(left) != 576 || len(right) != 576 {
		return fmt.Errorf("ms stereo requires 576 spectral lines: left=%d right=%d", len(left), len(right))
	}
	if maxPos < 0 {
		maxPos = 0
	}
	if maxPos > len(left) {
		maxPos = len(left)
	}
	for i := 0; i < maxPos; i++ {
		m := left[i]
		s := right[i]
		left[i] = (m + s) * MS_STEREO_SCALE
		right[i] = (m - s) * MS_STEREO_SCALE
	}
	return nil
}

func applyIntensityStereo(sampleRate uint16, gc *common.GranuleChannelInfo, scalefactors *maindata.Scalefactors, count1Start int, left []float32, right []float32) error {
	sfBands, ok := common.SCALEFACTOR_BAND_INDICES[sampleRate]
	if !ok {
		return fmt.Errorf("unsupported sample rate for intensity stereo: %d", sampleRate)
	}

	if gc.GetWindowSwitching() && gc.GetBlockType() == common.BlockTypeShort {
		if gc.GetMixedBlockFlag() {
			for sfb := 0; sfb < 8; sfb++ {
				if sfBands.Long[sfb] >= count1Start {
					applyIntensityLongBand(sfBands.Long, scalefactors.Long[sfb], sfb, left, right)
				}
			}
			for sfb := 3; sfb < 12; sfb++ {
				if sfBands.Short[sfb]*maindata.SCALEFACTOR_SHORT_WINDOW_COUNT >= count1Start {
					applyIntensityShortBand(sfBands.Short, scalefactors.Short[sfb], sfb, left, right)
				}
			}
			return nil
		}
		for sfb := 0; sfb < 12; sfb++ {
			if sfBands.Short[sfb]*maindata.SCALEFACTOR_SHORT_WINDOW_COUNT >= count1Start {
				applyIntensityShortBand(sfBands.Short, scalefactors.Short[sfb], sfb, left, right)
			}
		}
		return nil
	}

	for sfb := 0; sfb < 21; sfb++ {
		if sfBands.Long[sfb] >= count1Start {
			applyIntensityLongBand(sfBands.Long, scalefactors.Long[sfb], sfb, left, right)
		}
	}
	return nil
}

func applyIntensityLongBand(bands [23]int, isPos uint8, sfb int, left []float32, right []float32) {
	leftRatio, rightRatio, ok := intensityStereoFactors(isPos)
	if !ok {
		return
	}
	start := bands[sfb]
	stop := bands[sfb+1]
	for i := start; i < stop; i++ {
		left[i] *= leftRatio
		right[i] *= rightRatio
	}
}

func applyIntensityShortBand(bands [14]int, isPos [3]uint8, sfb int, left []float32, right []float32) {
	winLen := bands[sfb+1] - bands[sfb]
	for win := 0; win < maindata.SCALEFACTOR_SHORT_WINDOW_COUNT; win++ {
		leftRatio, rightRatio, ok := intensityStereoFactors(isPos[win])
		if !ok {
			continue
		}
		start := bands[sfb]*maindata.SCALEFACTOR_SHORT_WINDOW_COUNT + winLen*win
		stop := start + winLen
		for i := start; i < stop; i++ {
			left[i] *= leftRatio
			right[i] *= rightRatio
		}
	}
}

func intensityStereoFactors(isPos uint8) (float32, float32, bool) {
	if isPos >= 7 {
		return 0, 0, false
	}
	if isPos == 6 {
		return 1.0, 0.0, true
	}
	ratio := intensityStereoRatios[isPos]
	return ratio / (1.0 + ratio), 1.0 / (1.0 + ratio), true
}
