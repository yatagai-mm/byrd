package stereo

import (
	"github.com/yatagai-mm/byrd/internal/common"
	"github.com/yatagai-mm/byrd/internal/header"
	"github.com/yatagai-mm/byrd/internal/maindata"
	"math"
	"testing"
)

func TestApplyMSStereo(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[0] = 2
	right[0] = 0
	left[1] = 3
	right[1] = 1

	if err := ApplyMSStereo(left, right); err != nil {
		t.Fatalf("ApplyMSStereo failed: %v", err)
	}

	if !almostEqual(left[0], 2*MS_STEREO_SCALE) || !almostEqual(right[0], 2*MS_STEREO_SCALE) {
		t.Fatalf("line 0 got left=%f right=%f", left[0], right[0])
	}
	if !almostEqual(left[1], 4*MS_STEREO_SCALE) || !almostEqual(right[1], 2*MS_STEREO_SCALE) {
		t.Fatalf("line 1 got left=%f right=%f", left[1], right[1])
	}
}

func TestApplyJointStereo_ModeExtMSOnly(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[0] = 1
	right[0] = -1

	if err := ApplyJointStereo(44100, header.ChannelModeJointStereo, header.ModeExtensionMSStereo, &common.GranuleChannelInfo{}, &maindata.Scalefactors{}, left, right, 1, 1); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}

	if !almostEqual(left[0], 0) || !almostEqual(right[0], 2*MS_STEREO_SCALE) {
		t.Fatalf("got left=%f right=%f", left[0], right[0])
	}
}

func TestApplyJointStereo_NoOpWhenDisabled(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[0] = 5
	right[0] = 2

	if err := ApplyJointStereo(44100, header.ChannelModeStereo, header.ModeExtensionMSStereo, &common.GranuleChannelInfo{}, &maindata.Scalefactors{}, left, right, 1, 1); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}
	if left[0] != 5 || right[0] != 2 {
		t.Fatalf("non-joint stereo should be unchanged, got left=%f right=%f", left[0], right[0])
	}

	sfs := &maindata.Scalefactors{}
	sfs.Long[0] = 7
	if err := ApplyJointStereo(44100, header.ChannelModeJointStereo, header.ModeExtensionIntensityStereo, &common.GranuleChannelInfo{}, sfs, left, right, 0, 0); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}
	if left[0] != 5 || right[0] != 2 {
		t.Fatalf("joint stereo without ms flag should be unchanged, got left=%f right=%f", left[0], right[0])
	}
}

func TestApplyJointStereo_IntensityStereoLong(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[350] = 10
	left[300] = 3
	right[300] = 1

	sfs := &maindata.Scalefactors{}
	sfs.Long[20] = 6

	if err := ApplyJointStereo(44100, header.ChannelModeJointStereo, header.ModeExtensionIntensityStereo, &common.GranuleChannelInfo{}, sfs, left, right, 301, 301); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}

	if !almostEqual(left[350], 10) || !almostEqual(right[350], 0) {
		t.Fatalf("line 350 got left=%f right=%f", left[350], right[350])
	}
}

func TestApplyJointStereo_IntensityStereoShortMixed(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[48] = 12
	left[54] = 12
	left[35] = 2
	right[35] = 1

	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	gc.SetMixedBlockFlag(true)

	sfs := &maindata.Scalefactors{}
	sfs.Short[4][0] = 6
	sfs.Short[4][1] = 7

	if err := ApplyJointStereo(44100, header.ChannelModeJointStereo, header.ModeExtensionIntensityStereo, gc, sfs, left, right, 36, 36); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}

	if !almostEqual(left[48], 12) || !almostEqual(right[48], 0) {
		t.Fatalf("window 0 line got left=%f right=%f", left[48], right[48])
	}
	if !almostEqual(left[54], 12) || !almostEqual(right[54], 0) {
		t.Fatalf("window 1 line should remain unchanged, got left=%f right=%f", left[54], right[54])
	}
}

func TestApplyJointStereo_IntensityAndMS(t *testing.T) {
	left := make([]float32, 576)
	right := make([]float32, 576)
	left[350] = 4
	left[300] = 2
	right[300] = 1

	sfs := &maindata.Scalefactors{}
	sfs.Long[20] = 0

	if err := ApplyJointStereo(44100, header.ChannelModeJointStereo, header.ModeExtensionIntensityAndMS, &common.GranuleChannelInfo{}, sfs, left, right, 351, 351); err != nil {
		t.Fatalf("ApplyJointStereo failed: %v", err)
	}

	msLeft := (4 + 0) * MS_STEREO_SCALE
	msRight := (4 - 0) * MS_STEREO_SCALE
	wantLeft := msLeft
	wantRight := msRight
	if !almostEqual(left[350], wantLeft) || !almostEqual(right[350], wantRight) {
		t.Fatalf("line 350 got left=%f right=%f want left=%f right=%f", left[350], right[350], wantLeft, wantRight)
	}
}

func TestApplyMSStereo_InvalidLength(t *testing.T) {
	if err := ApplyMSStereo(make([]float32, 10), make([]float32, 576)); err == nil {
		t.Fatalf("expected invalid length error")
	}
}

func almostEqual(a, b float32) bool {
	const epsilon = 1e-5
	return math.Abs(float64(a-b)) <= epsilon
}
