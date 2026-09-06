package synthesis

import (
	"math/rand/v2"
	"testing"

	"github.com/yatagai-mm/byrd/internal/common"
)

// Exercise enough slots to wrap the full synthesis history several times.
func TestSynthesizeSubbandSamples_ExactOriginal(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	var state PolyphaseState
	var history [1024]float32
	for slot := 0; slot < 96; slot++ {
		var in, got [32]float32
		for k := range in {
			if slot < 64 {
				in[k] = (rng.Float32()*2 - 1) * 100
			}
		}
		if err := SynthesizeSubbandSamples(in[:], &state, got[:]); err != nil {
			t.Fatal(err)
		}
		copy(history[64:], history[:960])
		for i := range 64 {
			sum := synthesisMatrix[i][0] * in[0]
			for k := 1; k < 32; k++ {
				sum += synthesisMatrix[i][k] * in[k]
			}
			history[i] = sum
		}
		var u [512]float32
		for i := range 8 {
			copy(u[i*64:i*64+32], history[i*128:i*128+32])
			copy(u[i*64+32:i*64+64], history[i*128+96:i*128+128])
		}
		for j := range u {
			u[j] *= common.SynthDtbl[j]
		}
		for j := range got {
			want := u[j]
			for i := 1; i < 16; i++ {
				want += u[j+32*i]
			}
			if got[j] != want {
				t.Fatalf("slot=%d sample=%d got=%g want=%g", slot, j, got[j], want)
			}
		}
	}
}

func TestSynthesizeSubbandSamples_ZeroInput(t *testing.T) {
	var state PolyphaseState
	in := make([]float32, 32)
	out := make([]float32, 32)

	if err := SynthesizeSubbandSamples(in, &state, out); err != nil {
		t.Fatalf("SynthesizeSubbandSamples failed: %v", err)
	}
	for _, v := range out {
		if v != 0 {
			t.Fatalf("zero input should produce zero output, got %f", v)
		}
	}
}

func TestSynthesizeSubbandSamples_Stateful(t *testing.T) {
	var state PolyphaseState
	in := make([]float32, 32)
	in[0] = 1
	out := make([]float32, 32)

	if err := SynthesizeSubbandSamples(in, &state, out); err != nil {
		t.Fatalf("SynthesizeSubbandSamples failed: %v", err)
	}
	nonZero := 0
	for _, v := range out {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatalf("expected non-zero output")
	}

	zeroIn := make([]float32, 32)
	next := make([]float32, 32)
	if err := SynthesizeSubbandSamples(zeroIn, &state, next); err != nil {
		t.Fatalf("SynthesizeSubbandSamples second call failed: %v", err)
	}
	nonZeroNext := 0
	for _, v := range next {
		if v != 0 {
			nonZeroNext++
		}
	}
	if nonZeroNext == 0 {
		t.Fatalf("expected stateful non-zero output on second call")
	}
}

func TestSynthesizeGranule(t *testing.T) {
	var in [32][18]float32
	in[0][0] = 1
	in[1][1] = 1
	var state PolyphaseState
	var out [576]float32

	if err := SynthesizeGranule(&in, &state, &out); err != nil {
		t.Fatalf("SynthesizeGranule failed: %v", err)
	}

	nonZero := 0
	for _, v := range out {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatalf("expected non-zero granule output")
	}
}
