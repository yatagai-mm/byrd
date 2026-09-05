package hybrid

import (
	"github.com/yatagai-mm/byrd/internal/common"
	"testing"
)

func TestApplyAliasReduction_LongBlock(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	values := make([]float32, 576)
	values[17] = 2
	values[18] = 1

	if err := ApplyAliasReduction(gc, values); err != nil {
		t.Fatalf("ApplyAliasReduction failed: %v", err)
	}

	wantLower := 2*aliasReductionCS[0] - 1*aliasReductionCA[0]
	wantUpper := 1*aliasReductionCS[0] + 2*aliasReductionCA[0]
	if values[17] != wantLower || values[18] != wantUpper {
		t.Fatalf("boundary values got (%f,%f), want (%f,%f)", values[17], values[18], wantLower, wantUpper)
	}
}

func TestApplyAliasReduction_PureShortNoOp(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)

	values := make([]float32, 576)
	values[17] = 2
	values[18] = 1

	if err := ApplyAliasReduction(gc, values); err != nil {
		t.Fatalf("ApplyAliasReduction failed: %v", err)
	}
	if values[17] != 2 || values[18] != 1 {
		t.Fatalf("pure short block should be unchanged, got (%f,%f)", values[17], values[18])
	}
}

func TestApplyAliasReduction_MixedBlockFirstBoundaryOnly(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	gc.SetMixedBlockFlag(true)

	values := make([]float32, 576)
	values[17] = 2
	values[18] = 1
	values[35] = 4
	values[36] = 3

	if err := ApplyAliasReduction(gc, values); err != nil {
		t.Fatalf("ApplyAliasReduction failed: %v", err)
	}

	if values[35] != 4 || values[36] != 3 {
		t.Fatalf("mixed block should not alias-reduce second boundary, got (%f,%f)", values[35], values[36])
	}
}

func TestApplyAliasReduction_InvalidLength(t *testing.T) {
	if err := ApplyAliasReduction(&common.GranuleChannelInfo{}, make([]float32, 10)); err == nil {
		t.Fatalf("expected invalid length error")
	}
}
