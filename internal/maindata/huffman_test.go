package maindata

import (
	"fmt"
	"testing"

	"github.com/yatagai-mm/byrd/internal/common"
)

func sameTable(got *common.HuffmanTable, want common.HuffmanTable) bool {
	if got == nil {
		return false
	}
	if got.Linbits != want.Linbits || got.LUTBits != want.LUTBits || got.IsZero != want.IsZero {
		return false
	}
	if len(got.LUT) == 0 || len(want.LUT) == 0 {
		return len(got.LUT) == len(want.LUT)
	}
	return len(got.LUT) == len(want.LUT) && &got.LUT[0] == &want.LUT[0]
}

type lutCode struct {
	bits   uint32
	length int
	symbol uint16
}

func collectLUTCodes(t testing.TB, table common.HuffmanTable) []lutCode {
	t.Helper()
	if len(table.LUT) == 0 || table.LUTBits == 0 {
		t.Fatal("cannot collect codewords from an empty LUT table")
	}

	codes := make(map[lutCode]struct{})
	var walk func(offset int, width int, prefix uint32, prefixLength int, level int)
	walk = func(offset int, width int, prefix uint32, prefixLength int, level int) {
		if level >= 8 {
			t.Fatal("LUT table exceeds maximum lookup depth")
		}
		if width <= 0 || width > 7 {
			t.Fatalf("invalid LUT lookup width %d", width)
		}
		size := 1 << width
		if offset < 0 || offset+size > len(table.LUT) {
			t.Fatalf("LUT range [%d:%d] exceeds table length %d", offset, offset+size, len(table.LUT))
		}

		for index := 0; index < size; index++ {
			entry := table.LUT[offset+index]
			if entry == 0 {
				continue
			}

			leafBits := int(entry & 0xff)
			if leafBits != 0 {
				if leafBits > width {
					t.Fatalf("leaf width %d exceeds lookup width %d", leafBits, width)
				}
				segment := uint32(index >> (width - leafBits))
				code := lutCode{
					bits:   prefix<<leafBits | segment,
					length: prefixLength + leafBits,
					symbol: uint16((entry >> 8) & 0xff),
				}
				codes[code] = struct{}{}
				continue
			}

			nextOffset := int(entry >> 16)
			nextWidth := int((entry >> 8) & 0xff)
			walk(nextOffset, nextWidth, prefix<<width|uint32(index), prefixLength+width, level+1)
		}
	}
	walk(0, int(table.LUTBits), 0, 0, 0)

	result := make([]lutCode, 0, len(codes))
	for code := range codes {
		result = append(result, code)
	}
	return result
}

func lutCodeBits(code lutCode) []uint32 {
	bits := make([]uint32, code.length)
	for i := range bits {
		bits[i] = (code.bits >> (code.length - i - 1)) & 1
	}
	return bits
}

func findPairCode(t testing.TB, table common.HuffmanTable, wantX int, wantY int) []uint32 {
	t.Helper()
	for _, code := range collectLUTCodes(t, table) {
		x := int((code.symbol >> 4) & 0xf)
		y := int(code.symbol & 0xf)
		if x == wantX && y == wantY {
			return lutCodeBits(code)
		}
	}
	t.Fatalf("pair (%d,%d) not found in table", wantX, wantY)
	return nil
}

func findQuadCode(t testing.TB, table common.HuffmanTable, want [4]int) []uint32 {
	t.Helper()
	for _, code := range collectLUTCodes(t, table) {
		got := [4]int{
			int((code.symbol >> 3) & 0x1),
			int((code.symbol >> 2) & 0x1),
			int((code.symbol >> 1) & 0x1),
			int(code.symbol & 0x1),
		}
		if got == want {
			return lutCodeBits(code)
		}
	}
	t.Fatalf("quad %v not found in table", want)
	return nil
}

func TestSelectTable_LongBlock(t *testing.T) {
	gc := &common.GranuleChannelInfo{
		TableSelect:  [3]byte{5, 7, 9},
		Region0Count: 2,
		Region1Count: 3,
	}

	for _, tc := range []struct {
		lineIndex int
		wantTable int
	}{
		{0, 5},
		{11, 5},
		{12, 7},
		{29, 7},
		{30, 9},
	} {
		got, err := selectTable(44100, gc, tc.lineIndex)
		if err != nil {
			t.Fatalf("selectTable(%d) failed: %v", tc.lineIndex, err)
		}
		want := common.BaseTables[tc.wantTable]
		if !sameTable(got, want) {
			t.Fatalf("selectTable(%d) got table %+v, want table %d", tc.lineIndex, *got, tc.wantTable)
		}
	}
}

func TestSelectTable_LongBlock_Region1CountZero(t *testing.T) {
	gc := &common.GranuleChannelInfo{
		TableSelect:  [3]byte{5, 7, 9},
		Region0Count: 1,
		Region1Count: 0,
	}

	got, err := selectTable(48000, gc, 8)
	if err != nil {
		t.Fatalf("selectTable failed: %v", err)
	}
	want := common.BaseTables[7]
	if !sameTable(got, want) {
		t.Fatalf("line 8 got table %+v, want table 7", *got)
	}
}

func TestSelectTable_SwitchedWindow(t *testing.T) {
	gc := &common.GranuleChannelInfo{
		TableSelect:  [3]byte{16, 24, 0},
		Region0Count: common.PURE_SHORT_REGION0_COUNT,
		Region1Count: common.PURE_SHORT_REGION1_COUNT,
	}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeShort)

	got0, err := selectTable(44100, gc, 0)
	if err != nil {
		t.Fatalf("selectTable region0 failed: %v", err)
	}
	if got0.Linbits != 1 || len(got0.LUT) != len(common.BaseTables[16].LUT) {
		t.Fatalf("region0 got %+v, want table 16", *got0)
	}
	if !sameTable(got0, common.BaseTables[16]) {
		t.Fatalf("region0 got %+v, want table 16", *got0)
	}

	got1, err := selectTable(44100, gc, 36)
	if err != nil {
		t.Fatalf("selectTable region1 failed: %v", err)
	}
	if got1.Linbits != 4 || len(got1.LUT) != len(common.BaseTables[24].LUT) {
		t.Fatalf("region1 got %+v, want table 24", *got1)
	}
	if !sameTable(got1, common.BaseTables[24]) {
		t.Fatalf("region1 got %+v, want table 24", *got1)
	}
}

func TestSelectTable_StartBlock_UsesLongRegions(t *testing.T) {
	gc := &common.GranuleChannelInfo{
		TableSelect:  [3]byte{5, 7, 9},
		Region0Count: 2,
		Region1Count: 3,
	}
	gc.SetWindowSwitching(true)
	gc.SetBlockType(common.BlockTypeStart)

	for _, tc := range []struct {
		lineIndex int
		wantTable int
	}{
		{0, 5},
		{11, 5},
		{12, 7},
		{29, 7},
		{30, 9},
	} {
		got, err := selectTable(44100, gc, tc.lineIndex)
		if err != nil {
			t.Fatalf("selectTable(%d) failed: %v", tc.lineIndex, err)
		}
		want := common.BaseTables[tc.wantTable]
		if !sameTable(got, want) {
			t.Fatalf("selectTable(%d) got table %+v, want table %d", tc.lineIndex, *got, tc.wantTable)
		}
	}
}

func TestSelectTable_Invalid(t *testing.T) {
	if _, err := selectTable(44100, nil, 0); err == nil {
		t.Fatalf("expected nil granule channel error")
	}

	gc := &common.GranuleChannelInfo{TableSelect: [3]byte{4, 7, 9}}
	if _, err := selectTable(44100, gc, -1); err == nil {
		t.Fatalf("expected negative index error")
	}
	if _, err := selectTable(12345, gc, 0); err == nil {
		t.Fatalf("expected unsupported sample rate error")
	}
	if _, err := selectTable(44100, gc, 0); err == nil {
		t.Fatalf("expected unsupported table error")
	}
}

func TestGuardedReadBit_RespectsLimit(t *testing.T) {
	br := common.NewBitReader([]byte{0x80})
	var scratch uint32

	if err := guardedReadBit(br, 1, &scratch); err != nil {
		t.Fatalf("guardedReadBit failed: %v", err)
	}
	if scratch != 1 {
		t.Fatalf("scratch got %d, want 1", scratch)
	}
	if err := guardedReadBit(br, 1, &scratch); err == nil {
		t.Fatalf("expected limit error")
	}
}

func TestGuardedReadBits_RespectsLimit(t *testing.T) {
	br := common.NewBitReader([]byte{0xE0})
	var scratch uint32

	if err := guardedReadBits(br, 3, 3, &scratch); err != nil {
		t.Fatalf("guardedReadBits failed: %v", err)
	}
	if scratch != 0b111 {
		t.Fatalf("scratch got %b, want 111", scratch)
	}
	if err := guardedReadBits(br, 3, 1, &scratch); err == nil {
		t.Fatalf("expected limit error")
	}
}

func TestDecodeHuffmanPair_Table1(t *testing.T) {
	table := common.BaseTables[1]
	code := findPairCode(t, table, 1, 0)
	var bw bitWriter
	for _, bit := range code {
		bw.write(1, bit)
	}
	bw.write(1, 0) // x sign bit

	br := common.NewBitReader(bw.bytes())
	var scratch uint32
	x, y, err := decodeHuffmanPair(br, &table, len(code)+1, &scratch)
	if err != nil {
		t.Fatalf("decodeHuffmanPair failed: %v", err)
	}
	if x != 1 || y != 0 {
		t.Fatalf("decoded pair got (%d,%d), want (1,0)", x, y)
	}
}

func TestDecodeHuffmanPair_Table0ConsumesNoBits(t *testing.T) {
	table := common.BaseTables[0]
	br := common.NewBitReader(nil)
	var scratch uint32
	x, y, err := decodeHuffmanPair(br, &table, 0, &scratch)
	if err != nil {
		t.Fatalf("decodeHuffmanPair failed: %v", err)
	}
	if x != 0 || y != 0 {
		t.Fatalf("decoded pair got (%d,%d), want (0,0)", x, y)
	}
	if br.Pos != 0 {
		t.Fatalf("zero table consumed %d bits, want 0", br.Pos)
	}
}

func TestDecodeHuffmanPair_Linbits(t *testing.T) {
	table := common.BaseTables[16]
	code := findPairCode(t, table, 15, 15)
	var bw bitWriter
	for _, bit := range code {
		bw.write(1, bit)
	}
	bw.write(1, 0b1) // x linbit
	bw.write(1, 0b0) // x sign bit
	bw.write(1, 0b0) // y linbit
	bw.write(1, 0b0) // y sign bit

	br := common.NewBitReader(bw.bytes())
	var scratch uint32
	x, y, err := decodeHuffmanPair(br, &table, len(code)+4, &scratch)
	if err != nil {
		t.Fatalf("decodeHuffmanPair failed: %v", err)
	}
	if x != 16 || y != 15 {
		t.Fatalf("decoded pair got (%d,%d), want (16,15)", x, y)
	}
}

func TestDecodeHuffmanPair_Invalid(t *testing.T) {
	br := common.NewBitReader([]byte{0x00})
	var scratch uint32
	if _, _, err := decodeHuffmanPair(br, nil, 1, &scratch); err == nil {
		t.Fatalf("expected nil table error")
	}
	empty := common.HuffmanTable{}
	if _, _, err := decodeHuffmanPair(br, &empty, 1, &scratch); err == nil {
		t.Fatalf("expected empty table error")
	}
}

func TestDecodeHuffmanQuad_Table33(t *testing.T) {
	table := common.BaseTables[33]
	code := findQuadCode(t, table, [4]int{0, 1, 0, 1})
	var bw bitWriter
	for _, bit := range code {
		bw.write(1, bit)
	}
	bw.write(1, 0) // w sign bit
	bw.write(1, 0) // y sign bit

	br := common.NewBitReader(bw.bytes())
	var scratch uint32
	v, w, x, y, err := decodeHuffmanQuad(br, &table, len(code)+2, &scratch)
	if err != nil {
		t.Fatalf("decodeHuffmanQuad failed: %v", err)
	}
	if [4]int{v, w, x, y} != [4]int{0, 1, 0, 1} {
		t.Fatalf("decoded quad got %v, want [0 1 0 1]", [4]int{v, w, x, y})
	}
}

func TestDecodeHuffmanQuad_Invalid(t *testing.T) {
	br := common.NewBitReader([]byte{0x00})
	var scratch uint32
	if _, _, _, _, err := decodeHuffmanQuad(br, nil, 1, &scratch); err == nil {
		t.Fatalf("expected nil table error")
	}
	empty := common.HuffmanTable{}
	if _, _, _, _, err := decodeHuffmanQuad(br, &empty, 1, &scratch); err == nil {
		t.Fatalf("expected empty table error")
	}
}

func TestHuffmanLUTAllCodewords(t *testing.T) {
	expectedCodewords := map[int]int{
		1: 4,
		2: 9, 3: 9,
		5: 16, 6: 16,
		7: 36, 8: 36, 9: 36,
		10: 64, 11: 64, 12: 64,
		13: 256, 15: 256,
		16: 256, 17: 256, 18: 256, 19: 256,
		20: 256, 21: 256, 22: 256, 23: 256,
		24: 256, 25: 256, 26: 256, 27: 256,
		28: 256, 29: 256, 30: 256, 31: 256,
		32: 16, 33: 16,
	}

	for tableIndex := 1; tableIndex < len(common.BaseTables); tableIndex++ {
		table := &common.BaseTables[tableIndex]
		if len(table.LUT) == 0 {
			continue
		}

		t.Run(fmt.Sprintf("table-%d", tableIndex), func(t *testing.T) {
			codes := collectLUTCodes(t, *table)
			if got, want := len(codes), expectedCodewords[tableIndex]; got != want {
				t.Fatalf("collected %d codewords, want %d", got, want)
			}

			for _, code := range codes {
				var bw bitWriter
				bw.write(code.length, code.bits)
				reader := common.NewBitReader(bw.bytes())
				var scratch uint32
				got, err := decodeHuffmanSymbolLUT(reader, table, code.length, &scratch)
				if err != nil {
					t.Fatalf("code %0*b: %v", code.length, code.bits, err)
				}
				if got != code.symbol {
					t.Fatalf("code %0*b decoded %#x, want %#x", code.length, code.bits, got, code.symbol)
				}
				if reader.Pos != code.length {
					t.Fatalf("code %0*b consumed %d bits, want %d", code.length, code.bits, reader.Pos, code.length)
				}
			}
		})
	}
}

var benchmarkHuffmanSink int

func BenchmarkDecodeHuffmanPair(b *testing.B) {
	table := &common.BaseTables[13]
	pairs := [][2]int{
		{0, 0}, {1, 0}, {0, 1}, {1, 1},
		{3, 7}, {7, 3}, {10, 12}, {14, 14}, {15, 15},
	}

	var bw bitWriter
	bitLen := 0
	for range 128 {
		for _, pair := range pairs {
			code := findPairCode(b, *table, pair[0], pair[1])
			for _, bit := range code {
				bw.write(1, bit)
			}
			bitLen += len(code)
			if pair[0] != 0 {
				bw.write(1, 0)
				bitLen++
			}
			if pair[1] != 0 {
				bw.write(1, 0)
				bitLen++
			}
		}
	}
	data := bw.bytes()

	reader := common.NewBitReader(data)
	var scratch uint32
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if reader.Pos >= bitLen {
			reader.Pos = 0
		}
		x, y, err := decodeHuffmanPair(reader, table, bitLen, &scratch)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkHuffmanSink ^= x + y
	}
}

func TestParseBigValues_Table1(t *testing.T) {
	table := common.BaseTables[1]
	var bw bitWriter
	for _, bit := range findPairCode(t, table, 1, 0) {
		bw.write(1, bit)
	}
	bw.write(1, 1)
	for _, bit := range findPairCode(t, table, 0, 1) {
		bw.write(1, bit)
	}
	bw.write(1, 0)
	for _, bit := range findPairCode(t, table, 1, 1) {
		bw.write(1, bit)
	}
	bw.write(1, 1)
	bw.write(1, 0)

	br := common.NewBitReader(bw.bytes())
	gc := &common.GranuleChannelInfo{
		BigValues:    3,
		TableSelect:  [3]byte{1, 1, 1},
		Region0Count: 10,
		Region1Count: 10,
	}
	got := make([]int, 576)

	lines, err := ParseBigValues(br, 44100, gc, 12, &got)
	if err != nil {
		t.Fatalf("ParseBigValues failed: %v", err)
	}
	if lines != 6 {
		t.Fatalf("decoded line count = %d, want 6", lines)
	}
	want := []int{-1, 0, 0, 1, -1, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decoded big values prefix got %v, want %v", got[:len(want)], want)
		}
	}
}

func TestParseBigValues_RespectsPart23End(t *testing.T) {
	table := common.BaseTables[1]
	var bw bitWriter
	for _, bit := range findPairCode(t, table, 1, 0) {
		bw.write(1, bit)
	}
	bw.write(1, 1)

	br := common.NewBitReader(bw.bytes())
	gc := &common.GranuleChannelInfo{
		BigValues:    1,
		TableSelect:  [3]byte{1, 1, 1},
		Region0Count: 10,
		Region1Count: 10,
	}
	got := make([]int, 576)

	_, err := ParseBigValues(br, 44100, gc, 2, &got)
	if err == nil {
		t.Fatalf("expected part23 limit error, got nil")
	}
}

func TestParseCount1Values_Table33(t *testing.T) {
	table := common.BaseTables[33]
	var bw bitWriter
	for _, bit := range findQuadCode(t, table, [4]int{0, 1, 0, 1}) {
		bw.write(1, bit)
	}
	bw.write(1, 1)
	bw.write(1, 0)

	br := common.NewBitReader(bw.bytes())
	gc := &common.GranuleChannelInfo{}
	gc.SetCount1TableSelect(true)
	got := make([]int, 576)
	got[0] = 9
	got[1] = 8
	gc.BigValues = 1

	lines, err := ParseCount1Values(br, gc, 6, &got)
	if err != nil {
		t.Fatalf("ParseCount1Values failed: %v", err)
	}
	if lines != 4 {
		t.Fatalf("decoded line count = %d, want 4", lines)
	}
	want := []int{9, 8, 0, -1, 0, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decoded count1 values prefix got %v, want %v", got[:len(want)], want)
		}
	}
}

func TestParseCount1Values_RespectsPart23End(t *testing.T) {
	table := common.BaseTables[33]
	var bw bitWriter
	for _, bit := range findQuadCode(t, table, [4]int{1, 1, 1, 1}) {
		bw.write(1, bit)
	}

	br := common.NewBitReader(bw.bytes())
	gc := &common.GranuleChannelInfo{}
	gc.SetCount1TableSelect(true)
	got := make([]int, 576)

	lines, err := ParseCount1Values(br, gc, 3, &got)
	if err != nil {
		t.Fatalf("ParseCount1Values failed: %v", err)
	}
	if lines != 0 {
		t.Fatalf("decoded line count = %d, want 0", lines)
	}
}
