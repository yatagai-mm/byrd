package hybrid

import (
	"github.com/yatagai-mm/byrd/internal/common"
	"math"
	"testing"
)

func referenceIMDCTLong(in []float32, blockType common.BlockType) [36]float32 {
	var out [36]float32
	var window *[36]float32
	switch blockType {
	case common.BlockTypeStart:
		window = &startWindow
	case common.BlockTypeEnd:
		window = &endWindow
	default:
		window = &longWindow
	}
	for n := 0; n < 36; n++ {
		sum := 0.0
		for k := 0; k < 18; k++ {
			sum += float64(in[k]) * math.Cos(math.Pi/72*float64((2*n+19)*(2*k+1)))
		}
		out[n] = float32(sum) * window[n]
	}
	return out
}

func referenceIMDCTShort(in []float32) [36]float32 {
	var out [36]float32
	for win := 0; win < 3; win++ {
		for n := 0; n < 12; n++ {
			sum := 0.0
			for k := 0; k < 6; k++ {
				sum += float64(in[3*k+win]) * math.Cos(math.Pi/24*float64((2*n+7)*(2*k+1)))
			}
			out[6*win+n+6] += float32(sum) * shortWindow[n]
		}
	}
	return out
}

func assertClose36(t *testing.T, got [36]float32, want [36]float32, label string) {
	t.Helper()
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > 1e-5 {
			t.Fatalf("%s[%d] got %.12f, want %.12f", label, i, got[i], want[i])
		}
	}
}

func TestIMDCTLong_MatchesReference(t *testing.T) {
	in := make([]float32, 18)
	for i := range in {
		in[i] = float32(math.Sin(float64(i)+0.25) / 7)
	}
	var got [36]float32
	imdctLong(in, common.BlockTypeLong, &got)
	want := referenceIMDCTLong(in, common.BlockTypeLong)
	assertClose36(t, got, want, "imdctLong")
}

func TestIMDCTShort_MatchesReference(t *testing.T) {
	in := make([]float32, 18)
	for i := range in {
		in[i] = float32(math.Cos(float64(i)+0.5) / 9)
	}
	var got [36]float32
	imdctShort(in, &got)
	want := referenceIMDCTShort(in)
	assertClose36(t, got, want, "imdctShort")
}

func TestHybridSynthesis_ZeroInput(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	values := make([]float32, 576)
	var overlap [32][18]float32
	var out [32][18]float32

	if err := HybridSynthesis(gc, values, &overlap, &out); err != nil {
		t.Fatalf("HybridSynthesis failed: %v", err)
	}

	for sb := range out {
		for i := range out[sb] {
			if out[sb][i] != 0 || overlap[sb][i] != 0 {
				t.Fatalf("zero input should keep zero state, got out=%f overlap=%f", out[sb][i], overlap[sb][i])
			}
		}
	}
}

func TestHybridSynthesis_LongBlockOverlap(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	values := make([]float32, 576)
	values[0] = 1
	var overlap [32][18]float32
	var out [32][18]float32

	if err := HybridSynthesis(gc, values, &overlap, &out); err != nil {
		t.Fatalf("HybridSynthesis failed: %v", err)
	}

	nonZeroOut := 0
	nonZeroOverlap := 0
	for i := 0; i < 18; i++ {
		if out[0][i] != 0 {
			nonZeroOut++
		}
		if overlap[0][i] != 0 {
			nonZeroOverlap++
		}
	}
	if nonZeroOut == 0 || nonZeroOverlap == 0 {
		t.Fatalf("expected non-zero output and overlap, got out=%d overlap=%d", nonZeroOut, nonZeroOverlap)
	}

	values = make([]float32, 576)
	var next [32][18]float32
	if err := HybridSynthesis(gc, values, &overlap, &next); err != nil {
		t.Fatalf("HybridSynthesis second call failed: %v", err)
	}
	nonZeroNext := 0
	for i := 0; i < 18; i++ {
		if next[0][i] != 0 {
			nonZeroNext++
		}
	}
	if nonZeroNext == 0 {
		t.Fatalf("expected overlap-add output on second call")
	}
}

func TestHybridSynthesis_ShortBlock(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	values := make([]float32, 576)
	values[0] = 1
	values[1] = 2
	values[2] = 3
	var overlap [32][18]float32
	var out [32][18]float32

	if err := HybridSynthesis(gc, values, &overlap, &out); err != nil {
		t.Fatalf("HybridSynthesis failed: %v", err)
	}

	nonZero := 0
	for i := 0; i < 18; i++ {
		if out[0][i] != 0 || overlap[0][i] != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatalf("expected non-zero short block synthesis")
	}
}

func TestHybridSynthesis_MixedBlockUsesLongForFirstSubbands(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	gc.SetMixedBlockFlag(true)
	values := make([]float32, 576)
	values[17] = 1
	values[35] = 1
	values[36] = 1
	var overlap [32][18]float32
	var out [32][18]float32

	if err := HybridSynthesis(gc, values, &overlap, &out); err != nil {
		t.Fatalf("HybridSynthesis failed: %v", err)
	}

	if out[0][0] == 0 && out[1][0] == 0 && out[2][0] == 0 {
		t.Fatalf("expected mixed block synthesis to produce output")
	}
}
