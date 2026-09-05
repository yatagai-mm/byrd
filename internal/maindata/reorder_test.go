package maindata

import (
	"github.com/yatagai-mm/byrd/internal/common"
	"testing"
)

func TestReorder_LongBlock_NoOp(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	in := make([]float32, 576)
	out := make([]float32, 576)
	for i := range in {
		in[i] = float32(i)
	}

	if err := Reorder(48000, gc, in, &out); err != nil {
		t.Fatalf("Reorder failed: %v", err)
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("line %d got %f, want %f", i, out[i], in[i])
		}
	}
}

func TestReorder_ShortBlock_InterleavesWindows(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	in := make([]float32, 576)
	out := make([]float32, 576)
	for i := 0; i < 12; i++ {
		in[i] = float32(i + 1)
	}

	if err := Reorder(48000, gc, in, &out); err != nil {
		t.Fatalf("Reorder failed: %v", err)
	}

	want := []float32{1, 5, 9, 2, 6, 10, 3, 7, 11, 4, 8, 12}
	for i, v := range want {
		if out[i] != v {
			t.Fatalf("line %d got %f, want %f", i, out[i], v)
		}
	}
}

func TestReorder_MixedBlock_PreservesLongRegion(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)
	gc.SetMixedBlockFlag(true)
	in := make([]float32, 576)
	out := make([]float32, 576)
	for i := 0; i < mixedLongEndLine; i++ {
		in[i] = float32(i + 1)
	}
	for i := 0; i < 12; i++ {
		in[mixedLongEndLine+i] = float32(100 + i)
	}

	if err := Reorder(48000, gc, in, &out); err != nil {
		t.Fatalf("Reorder failed: %v", err)
	}

	for i := 0; i < mixedLongEndLine; i++ {
		if out[i] != in[i] {
			t.Fatalf("long region line %d got %f, want %f", i, out[i], in[i])
		}
	}

	want := []float32{100, 104, 108, 101, 105, 109, 102, 106, 110, 103, 107, 111}
	for i, v := range want {
		line := mixedLongEndLine + i
		if out[line] != v {
			t.Fatalf("mixed short line %d got %f, want %f", line, out[line], v)
		}
	}
}

func TestReorder_InvalidOutputLength(t *testing.T) {
	gc := &common.GranuleChannelInfo{}
	out := make([]float32, 0, 576)
	err := Reorder(48000, gc, make([]float32, 576), &out)
	if err == nil {
		t.Fatalf("expected invalid output length error")
	}
}
