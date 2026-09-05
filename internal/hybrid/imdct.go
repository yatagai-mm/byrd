package hybrid

import (
	"fmt"
	"math"

	"github.com/yatagai-mm/byrd/internal/common"
)

var longWindow = buildLongWindow()
var startWindow = buildStartWindow()
var shortWindow = buildShortWindow()
var endWindow = buildEndWindow()
var imdctLongTable = buildIMDCTLongTable()
var imdctShortTable = buildIMDCTShortTable()

func buildLongWindow() [36]float32 {
	var w [36]float32
	for i := range w {
		w[i] = float32(math.Sin(math.Pi / 36 * (float64(i) + 0.5)))
	}
	return w
}

func buildStartWindow() [36]float32 {
	var w [36]float32
	for i := 0; i < 18; i++ {
		w[i] = float32(math.Sin(math.Pi / 36 * (float64(i) + 0.5)))
	}
	for i := 18; i < 24; i++ {
		w[i] = 1
	}
	for i := 24; i < 30; i++ {
		w[i] = float32(math.Sin(math.Pi / 12 * (float64(i-18) + 0.5)))
	}
	return w
}

func buildShortWindow() [12]float32 {
	var w [12]float32
	for i := range w {
		w[i] = float32(math.Sin(math.Pi / 12 * (float64(i) + 0.5)))
	}
	return w
}

func buildEndWindow() [36]float32 {
	var w [36]float32
	for i := 6; i < 12; i++ {
		w[i] = float32(math.Sin(math.Pi / 12 * (float64(i-6) + 0.5)))
	}
	for i := 12; i < 18; i++ {
		w[i] = 1
	}
	for i := 18; i < 36; i++ {
		w[i] = float32(math.Sin(math.Pi / 36 * (float64(i) + 0.5)))
	}
	return w
}

func buildIMDCTLongTable() [36][18]float32 {
	var table [36][18]float32
	for n := range 36 {
		for k := range 18 {
			table[n][k] = float32(math.Cos(math.Pi / 72 * float64((2*n+19)*(2*k+1))))
		}
	}
	return table
}

func buildIMDCTShortTable() [12][6]float32 {
	var table [12][6]float32
	for n := range 12 {
		for k := range 6 {
			table[n][k] = float32(math.Cos(math.Pi / 24 * float64((2*n+7)*(2*k+1))))
		}
	}
	return table
}

func blockTypeForSubband(gc *common.GranuleChannelInfo, sb int) common.BlockType {
	if !gc.GetWindowSwitching() {
		return common.BlockTypeLong
	}
	if gc.GetMixedBlockFlag() && sb < 2 {
		return common.BlockTypeLong
	}
	return gc.GetBlockType()
}

func imdctLong(in []float32, blockType common.BlockType, out *[36]float32) {
	var window *[36]float32
	switch blockType {
	case common.BlockTypeStart:
		window = &startWindow
	case common.BlockTypeEnd:
		window = &endWindow
	default:
		window = &longWindow
	}

	for n := range 36 {
		var sum float32
		for k := range 18 {
			sum += in[k] * imdctLongTable[n][k]
		}
		out[n] = sum * window[n]
	}
}

func imdctShort(in []float32, out *[36]float32) {
	clear(out[:])
	for win := 0; win < 3; win++ {
		for n := 0; n < 12; n++ {
			var sum float32
			for k := 0; k < 6; k++ {
				sum += in[3*k+win] * imdctShortTable[n][k]
			}
			out[6*win+n+6] += sum * shortWindow[n]
		}
	}
}

func HybridSynthesis(gc *common.GranuleChannelInfo, values []float32, overlap *[32][18]float32, out *[32][18]float32) error {
	if gc == nil {
		return fmt.Errorf("nil granule channel info")
	}
	if len(values) != 576 {
		return fmt.Errorf("hybrid synthesis requires 576 spectral lines: got %d", len(values))
	}
	if overlap == nil {
		return fmt.Errorf("nil overlap state")
	}
	if out == nil {
		return fmt.Errorf("nil hybrid output")
	}

	for sb := 0; sb < 32; sb++ {
		var tmp [36]float32
		subband := values[sb*18 : (sb+1)*18]
		switch blockTypeForSubband(gc, sb) {
		case common.BlockTypeShort:
			imdctShort(subband, &tmp)
		case common.BlockTypeStart, common.BlockTypeEnd, common.BlockTypeLong:
			imdctLong(subband, blockTypeForSubband(gc, sb), &tmp)
		default:
			return fmt.Errorf("unsupported block type: %v", gc.GetBlockType())
		}
		for i := 0; i < 18; i++ {
			out[sb][i] = tmp[i] + overlap[sb][i]
			overlap[sb][i] = tmp[i+18]
		}
	}

	return nil
}
