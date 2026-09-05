package common

import "testing"

func TestBaseTables(t *testing.T) {
	if got := BaseTables[0]; !got.IsZero || len(got.LUT) != 0 || got.LUTBits != 0 || got.Linbits != 0 {
		t.Fatalf("table 0 got %+v, want zero table", got)
	}

	for i := 1; i <= 33; i++ {
		if i == 4 || i == 14 {
			continue
		}
		if got := BaseTables[i]; len(got.LUT) == 0 || got.LUTBits == 0 || got.IsZero {
			t.Fatalf("table %d got %+v, want non-empty LUT table", i, got)
		}
	}

	if got := BaseTables[4]; len(got.LUT) != 0 || got.LUTBits != 0 || got.Linbits != 0 || got.IsZero {
		t.Fatalf("table 4 got %+v, want unsupported table", got)
	}
	if got := BaseTables[14]; len(got.LUT) != 0 || got.LUTBits != 0 || got.Linbits != 0 || got.IsZero {
		t.Fatalf("table 14 got %+v, want unsupported table", got)
	}

	if len(BaseTables[16].LUT) == 0 || &BaseTables[16].LUT[0] != &BaseTables[23].LUT[0] {
		t.Fatalf("table 16/23 LUT table is not shared")
	}
	if len(BaseTables[24].LUT) == 0 || &BaseTables[24].LUT[0] != &BaseTables[31].LUT[0] {
		t.Fatalf("table 24/31 LUT table is not shared")
	}

	for _, tc := range []struct {
		table   int
		linbits int
	}{
		{16, 1},
		{17, 2},
		{23, 13},
		{24, 4},
		{31, 13},
		{32, 0},
		{33, 0},
	} {
		if got := BaseTables[tc.table].Linbits; got != tc.linbits {
			t.Fatalf("table %d linbits got %d, want %d", tc.table, got, tc.linbits)
		}
	}

	for _, tc := range []struct {
		table    int
		entries  int
		rootBits uint8
	}{
		{1, 8, 3},
		{2, 64, 6},
		{16, 662, 7},
		{24, 414, 7},
		{32, 64, 6},
		{33, 16, 4},
	} {
		got := BaseTables[tc.table]
		if len(got.LUT) != tc.entries {
			t.Fatalf("table %d LUT entries got %d, want %d", tc.table, len(got.LUT), tc.entries)
		}
		if got.LUTBits != tc.rootBits {
			t.Fatalf("table %d root bits got %d, want %d", tc.table, got.LUTBits, tc.rootBits)
		}
	}
}

func TestScalefactorBandIndices(t *testing.T) {
	for _, tc := range []struct {
		sampleRate uint16
		long8      int
		long21     int
		short3     int
		short12    int
	}{
		{32000, 36, 550, 12, 136},
		{44100, 36, 418, 12, 136},
		{48000, 36, 384, 12, 126},
	} {
		bands, ok := SCALEFACTOR_BAND_INDICES[tc.sampleRate]
		if !ok {
			t.Fatalf("missing scalefactor band indices for %d", tc.sampleRate)
		}
		if bands.Long[8] != tc.long8 || bands.Long[21] != tc.long21 {
			t.Fatalf("sampleRate %d long bands got [%d %d], want [%d %d]", tc.sampleRate, bands.Long[8], bands.Long[21], tc.long8, tc.long21)
		}
		if bands.Short[3] != tc.short3 || bands.Short[12] != tc.short12 {
			t.Fatalf("sampleRate %d short bands got [%d %d], want [%d %d]", tc.sampleRate, bands.Short[3], bands.Short[12], tc.short3, tc.short12)
		}
		if bands.Long[22] != 576 || bands.Short[13] != 192 {
			t.Fatalf("sampleRate %d terminal bands got long=%d short=%d, want long=576 short=192", tc.sampleRate, bands.Long[22], bands.Short[13])
		}
	}
}
