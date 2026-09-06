package maindata

import (
	"github.com/yatagai-mm/byrd/internal/common"
	"math"
	"testing"
)

func TestSignedPow43_ExactOriginal(t *testing.T) {
	// 15 plus the largest 13-bit escape value is the MP3 Huffman maximum.
	for is := -8207; is <= 8207; is++ {
		mag := float32(math.Pow(math.Abs(float64(is)), 4.0/3.0))
		want := float32(math.Copysign(float64(mag), float64(is)))
		if got := signedPow43(is); math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("is=%d got=%g want=%g", is, got, want)
		}
	}
	for _, is := range []int{math.MinInt, math.MaxInt} {
		mag := float32(math.Pow(math.Abs(float64(is)), 4.0/3.0))
		want := float32(math.Copysign(float64(mag), float64(is)))
		if got := signedPow43(is); got != want {
			t.Fatalf("is=%d got=%g want=%g", is, got, want)
		}
	}
}

func TestRequantizeScale_ExactOriginal(t *testing.T) {
	// Include the fallback range for every value representable by the
	// uint8 scalefactor/subblock fields, not only valid MPEG-1 bit fields.
	for q := -46; q <= 3271; q++ {
		want := float32(math.Pow(2, -float64(q)/4))
		if got := requantizeScale(q); math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("q=%d got=%g want=%g", q, got, want)
		}
	}
}

func TestRequantize_LongBlock_ZeroInput(t *testing.T) {
	gc := &common.GranuleChannelInfo{GlobalGain: 210}
	out := make([]float32, 576)
	if err := Requantize(44100, gc, &Scalefactors{}, make([]int, 576), &out); err != nil {
		t.Fatalf("Requantize failed: %v", err)
	}
	for i, v := range out {
		if v != 0 {
			t.Fatalf("line %d got %f, want 0", i, v)
		}
	}
}

func TestRequantize_LongBlock_UsesPreflag(t *testing.T) {
	gc := &common.GranuleChannelInfo{GlobalGain: 220}
	gc.SetPreflag(true)
	sfs := &Scalefactors{}
	sfs.Long[11] = 2
	spectral := make([]int, 576)
	spectral[62] = 3
	out := make([]float32, 576)
	if err := Requantize(44100, gc, sfs, spectral, &out); err != nil {
		t.Fatalf("Requantize failed: %v", err)
	}
	q := 210 - 220 + 2*(1)*(2+1)
	want := math.Pow(3, 4.0/3.0) * math.Pow(2, -float64(q)/4.0)
	if math.Abs(float64(out[62])-want) > 1e-5 {
		t.Fatalf("line 62 got %f, want %f", out[62], want)
	}
}

func TestRequantize_ShortBlock_UsesSubblockGain(t *testing.T) {
	gc := &common.GranuleChannelInfo{GlobalGain: 210, SubblockGain: [3]byte{0, 2, 0}}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	sfs := &Scalefactors{}
	sfs.Short[0][1] = 3
	spectral := make([]int, 576)
	spectral[5] = -2
	out := make([]float32, 576)
	if err := Requantize(44100, gc, sfs, spectral, &out); err != nil {
		t.Fatalf("Requantize failed: %v", err)
	}
	q := 210 - 210 + 8*2 + 2*(1)*3
	want := -math.Pow(2, 4.0/3.0) * math.Pow(2, -float64(q)/4.0)
	if math.Abs(float64(out[5])-want) > 1e-5 {
		t.Fatalf("line 5 got %f, want %f", out[5], want)
	}
}

func TestRequantize_InvalidInputLength(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	out := make([]float32, 576)
	err := Requantize(44100, gc, &Scalefactors{}, []int{1}, &out)
	if err == nil {
		t.Fatalf("expected invalid input length error")
	}
}

func TestRequantize_InvalidOutputLength(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	out := make([]float32, 0, 576)
	err := Requantize(44100, gc, &Scalefactors{}, make([]int, 576), &out)
	if err == nil {
		t.Fatalf("expected invalid output length error")
	}
}
