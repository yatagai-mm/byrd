package synthesis

import (
	"fmt"
	"math"

	"github.com/yatagai-mm/byrd/internal/common"
)

type PolyphaseState struct {
	v [1024]float32
}

var synthesisMatrix = buildSynthesisMatrix()

func buildSynthesisMatrix() [64][32]float32 {
	var m [64][32]float32
	for i := range 64 {
		for k := range 32 {
			m[i][k] = float32(math.Cos(math.Pi / 64 * float64((i+16)*(2*k+1))))
		}
	}
	return m
}

func dotProduct32(a *[32]float32, b *[32]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3] +
		a[4]*b[4] + a[5]*b[5] + a[6]*b[6] + a[7]*b[7] +
		a[8]*b[8] + a[9]*b[9] + a[10]*b[10] + a[11]*b[11] +
		a[12]*b[12] + a[13]*b[13] + a[14]*b[14] + a[15]*b[15] +
		a[16]*b[16] + a[17]*b[17] + a[18]*b[18] + a[19]*b[19] +
		a[20]*b[20] + a[21]*b[21] + a[22]*b[22] + a[23]*b[23] +
		a[24]*b[24] + a[25]*b[25] + a[26]*b[26] + a[27]*b[27] +
		a[28]*b[28] + a[29]*b[29] + a[30]*b[30] + a[31]*b[31]
}

func sumWindowColumn16(u *[512]float32, column int) float32 {
	return u[column] + u[column+32] + u[column+64] + u[column+96] +
		u[column+128] + u[column+160] + u[column+192] + u[column+224] +
		u[column+256] + u[column+288] + u[column+320] + u[column+352] +
		u[column+384] + u[column+416] + u[column+448] + u[column+480]
}

func SynthesizeSubbandSamples(in []float32, state *PolyphaseState, out []float32) error {
	if len(in) != 32 {
		return fmt.Errorf("polyphase synthesis requires 32 subband samples: got %d", len(in))
	}
	if state == nil {
		return fmt.Errorf("nil polyphase state")
	}
	if len(out) != 32 {
		return fmt.Errorf("polyphase synthesis requires 32 output samples: got %d", len(out))
	}

	var x [64]float32
	inVec := (*[32]float32)(in)
	// Cosine symmetry leaves only 33 independent rows. Retain row 16's
	// original dot product: its coefficients are mathematically zero, but
	// the generated table contains small nonzero floating-point values.
	for i := range 16 {
		x[i] = dotProduct32(&synthesisMatrix[i], inVec)
		x[32-i] = -x[i]
		row := i + 33
		x[row] = dotProduct32(&synthesisMatrix[row], inVec)
		x[96-row] = x[row]
	}
	x[16] = dotProduct32(&synthesisMatrix[16], inVec)

	copy(state.v[64:], state.v[:960])
	copy(state.v[:64], x[:])

	var u [512]float32
	for i := 0; i < 8; i++ {
		copy(u[i*64:i*64+32], state.v[i*128:i*128+32])
		copy(u[i*64+32:i*64+64], state.v[i*128+96:i*128+128])
	}

	for j := range u {
		u[j] *= common.SynthDtbl[j]
	}

	uVec := &u
	for j := range 32 {
		out[j] = sumWindowColumn16(uVec, j)
	}

	return nil
}

func SynthesizeGranule(in *[32][18]float32, state *PolyphaseState, out *[576]float32) error {
	if in == nil {
		return fmt.Errorf("nil hybrid input")
	}
	if state == nil {
		return fmt.Errorf("nil polyphase state")
	}
	if out == nil {
		return fmt.Errorf("nil pcm output")
	}

	var subbandIn [32]float32
	var slotOut [32]float32
	for ss := 0; ss < 18; ss++ {
		for sb := 0; sb < 32; sb++ {
			subbandIn[sb] = in[sb][ss]
		}
		if err := SynthesizeSubbandSamples(subbandIn[:], state, slotOut[:]); err != nil {
			return err
		}
		copy(out[ss*32:(ss+1)*32], slotOut[:])
	}

	return nil
}
