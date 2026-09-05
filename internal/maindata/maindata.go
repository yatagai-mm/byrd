package maindata

import (
	"fmt"
	"strings"

	"github.com/yatagai-mm/byrd/internal/common"
)

const (
	SCALEFACTOR_LONG_SFB_0_START = 0
	SCALEFACTOR_LONG_SFB_0_END   = 5
	SCALEFACTOR_LONG_SFB_1_START = 6
	SCALEFACTOR_LONG_SFB_1_END   = 10
	SCALEFACTOR_LONG_SFB_2_START = 11
	SCALEFACTOR_LONG_SFB_2_END   = 15
	SCALEFACTOR_LONG_SFB_3_START = 16
	SCALEFACTOR_LONG_SFB_3_END   = 20

	SCALEFACTOR_MIXED_LONG_START      = 0
	SCALEFACTOR_MIXED_LONG_END        = 7
	SCALEFACTOR_MIXED_SHORT_LOW_START = 3
	SCALEFACTOR_MIXED_SHORT_LOW_END   = 5

	SCALEFACTOR_SHORT_LOW_START  = 0
	SCALEFACTOR_SHORT_LOW_END    = 5
	SCALEFACTOR_SHORT_HIGH_START = 6
	SCALEFACTOR_SHORT_HIGH_END   = 11

	SCALEFACTOR_SHORT_WINDOW_COUNT = 3
)

const (
	reservoirBufferSize = 512
	reservoirBufferMask = reservoirBufferSize - 1
	RESERVOIR_MAX       = reservoirBufferSize - 1 // 2^9 - 1, the maximum main_data_begin value
)

// Reservoir keeps the most recent main-data bytes needed by a later frame.
// The backing array is a power-of-two ring, while the logical size is limited
// to the MPEG-1 Layer III maximum backwards offset of 511 bytes.
type Reservoir struct {
	data     [reservoirBufferSize]byte
	writePos uint16
	size     uint16
}

func (r *Reservoir) Len() int {
	return int(r.size)
}

func (r *Reservoir) copyLast(dst []byte, n int) {
	start := (int(r.writePos) - n) & reservoirBufferMask
	first := min(n, reservoirBufferSize-start)
	copy(dst[:first], r.data[start:start+first])
	copy(dst[first:n], r.data[:n-first])
}

func (r *Reservoir) append(src []byte) {
	if len(src) == 0 {
		return
	}
	if len(src) >= RESERVOIR_MAX {
		src = src[len(src)-RESERVOIR_MAX:]
		copy(r.data[:RESERVOIR_MAX], src)
		r.writePos = RESERVOIR_MAX
		r.size = RESERVOIR_MAX
		return
	}

	writePos := int(r.writePos)
	first := min(len(src), reservoirBufferSize-writePos)
	copy(r.data[writePos:writePos+first], src[:first])
	copy(r.data[:len(src)-first], src[first:])
	r.writePos = uint16((writePos + len(src)) & reservoirBufferMask)
	r.size = uint16(min(RESERVOIR_MAX, int(r.size)+len(src)))
}

// generate main data from reservoir offset
func ReadMainData(mainDataBegin uint16, mainDataReservoir *Reservoir, cur []byte, mainData *[]byte) error {
	if mainDataReservoir == nil {
		return fmt.Errorf("nil main data reservoir")
	}
	if mainData == nil {
		return fmt.Errorf("nil main data buffer")
	}
	// mainDataBegin is the reverse offset from the end of the reservoir, so it can't be larger than the reservoir itself
	if int(mainDataBegin) > mainDataReservoir.Len() {
		return fmt.Errorf("bit reservoir underflow: need %d bytes, have %d", mainDataBegin, mainDataReservoir.Len())
	}
	mainDataLen := int(mainDataBegin) + len(cur)

	// we reuse mainData buffer for reducing GC overhead, but grow it if needed
	if cap(*mainData) < mainDataLen {
		*mainData = make([]byte, mainDataLen)
	} else {
		*mainData = (*mainData)[:mainDataLen]
	}
	mainDataReservoir.copyLast((*mainData)[:mainDataBegin], int(mainDataBegin))
	copy((*mainData)[mainDataBegin:], cur)
	mainDataReservoir.append(cur)

	return nil
}

type Scalefactors struct {
	Long  [21]uint8    // 21 bands (11 for slen1 and 10 for slen2) for long blocks
	Short [12][3]uint8 // 12 bands for short blocks, each with 3 windows
}

func readScalefactorBits(br *common.BitReader, limit int, width int, scratch *uint32) (uint8, error) {
	if width == 0 {
		return 0, nil
	}
	if br.Pos+width > limit {
		return 0, fmt.Errorf("scalefactors exceed part23 length: need %d more bits, have %d", width, limit-br.Pos)
	}
	if err := br.ReadBitsTo(scratch, width); err != nil {
		return 0, err
	}
	return uint8(*scratch), nil
}

func readLongScalefactorRange(br *common.BitReader, limit int, width int, from int, to int, dst *Scalefactors, scratch *uint32) error {
	for sfb := from; sfb <= to; sfb++ {
		v, err := readScalefactorBits(br, limit, width, scratch)
		if err != nil {
			return err
		}
		dst.Long[sfb] = v
	}
	return nil
}

func readShortScalefactorRange(br *common.BitReader, limit int, width int, from int, to int, dst *Scalefactors, scratch *uint32) error {
	for sfb := from; sfb <= to; sfb++ {
		for win := range SCALEFACTOR_SHORT_WINDOW_COUNT {
			v, err := readScalefactorBits(br, limit, width, scratch)
			if err != nil {
				return err
			}
			dst.Short[sfb][win] = v
		}
	}
	return nil
}

func ParseScaleFactor(br *common.BitReader, gc *common.GranuleChannelInfo, scfsi [4]byte, granule int, prev *Scalefactors, scaleFactors *Scalefactors) (int, error) {
	if br == nil {
		return 0, fmt.Errorf("nil BitReader")
	}
	if gc == nil {
		return 0, fmt.Errorf("nil granule channel info")
	}
	if scaleFactors == nil {
		return 0, fmt.Errorf("nil scalefactors")
	}
	if granule < 0 || granule > 1 {
		return 0, fmt.Errorf("invalid granule index: %d", granule)
	}
	if int(gc.ScalefacCompress) >= len(common.SCALEFACTOR_COMPRESS) {
		return 0, fmt.Errorf("invalid scalefactor_compress: %d", gc.ScalefacCompress)
	}

	*scaleFactors = Scalefactors{}
	start := br.Pos
	limit := start + int(gc.Part23Length)
	slen := common.SCALEFACTOR_COMPRESS[gc.ScalefacCompress]
	var scratch uint32

	longSyntax := !gc.GetWindowSwitching() || common.BlockType(gc.GetBlockType()) != common.BlockTypeShort
	switch {
	case longSyntax:
		groups := [...]struct {
			from, to int
			width    int
		}{
			{SCALEFACTOR_LONG_SFB_0_START, SCALEFACTOR_LONG_SFB_0_END, slen.Slen1},
			{SCALEFACTOR_LONG_SFB_1_START, SCALEFACTOR_LONG_SFB_1_END, slen.Slen1},
			{SCALEFACTOR_LONG_SFB_2_START, SCALEFACTOR_LONG_SFB_2_END, slen.Slen2},
			{SCALEFACTOR_LONG_SFB_3_START, SCALEFACTOR_LONG_SFB_3_END, slen.Slen2},
		}
		for i, group := range groups {
			if granule == 1 && scfsi[i] == 1 {
				if prev == nil {
					return 0, fmt.Errorf("missing previous granule scalefactors for scfsi reuse")
				}
				copy(scaleFactors.Long[group.from:group.to+1], prev.Long[group.from:group.to+1])
				continue
			}
			if err := readLongScalefactorRange(br, limit, group.width, group.from, group.to, scaleFactors, &scratch); err != nil {
				return 0, err
			}
		}
	case gc.GetMixedBlockFlag():
		if err := readLongScalefactorRange(br, limit, slen.Slen1, SCALEFACTOR_MIXED_LONG_START, SCALEFACTOR_MIXED_LONG_END, scaleFactors, &scratch); err != nil {
			return 0, err
		}
		if err := readShortScalefactorRange(br, limit, slen.Slen1, SCALEFACTOR_MIXED_SHORT_LOW_START, SCALEFACTOR_MIXED_SHORT_LOW_END, scaleFactors, &scratch); err != nil {
			return 0, err
		}
		if err := readShortScalefactorRange(br, limit, slen.Slen2, SCALEFACTOR_SHORT_HIGH_START, SCALEFACTOR_SHORT_HIGH_END, scaleFactors, &scratch); err != nil {
			return 0, err
		}
	default:
		if err := readShortScalefactorRange(br, limit, slen.Slen1, SCALEFACTOR_SHORT_LOW_START, SCALEFACTOR_SHORT_LOW_END, scaleFactors, &scratch); err != nil {
			return 0, err
		}
		if err := readShortScalefactorRange(br, limit, slen.Slen2, SCALEFACTOR_SHORT_HIGH_START, SCALEFACTOR_SHORT_HIGH_END, scaleFactors, &scratch); err != nil {
			return 0, err
		}
	}

	return br.Pos - start, nil
}

func ParseBigValues(br *common.BitReader, sampleRate uint16, gc *common.GranuleChannelInfo, part23EndBit int, spectralValues *[]int) (int, error) {
	if br == nil {
		return 0, fmt.Errorf("nil BitReader")
	}
	if gc == nil {
		return 0, fmt.Errorf("nil granule channel info")
	}
	if spectralValues == nil {
		return 0, fmt.Errorf("nil spectral values buffer")
	}

	lineCount := int(gc.BigValues) * 2
	if lineCount > 576 {
		return 0, fmt.Errorf("invalid big_values: %d", gc.BigValues)
	}
	if cap(*spectralValues) < 576 {
		*spectralValues = make([]int, 576)
	} else {
		*spectralValues = (*spectralValues)[:576]
	}
	clear(*spectralValues)

	var scratch uint32
	for line := 0; line < lineCount; line += 2 {
		table, err := selectTable(sampleRate, gc, line)
		if err != nil {
			return line, err
		}
		x, y, err := decodeHuffmanPair(br, table, part23EndBit, &scratch)
		if err != nil {
			return line, err
		}

		(*spectralValues)[line] = x
		(*spectralValues)[line+1] = y
	}

	return lineCount, nil
}

func ParseCount1Values(br *common.BitReader, gc *common.GranuleChannelInfo, part23EndBit int, spectralValues *[]int) (int, error) {
	if br == nil {
		return 0, fmt.Errorf("nil BitReader")
	}
	if gc == nil {
		return 0, fmt.Errorf("nil granule channel info")
	}
	if spectralValues == nil {
		return 0, fmt.Errorf("nil spectral values buffer")
	}

	tableIndex := 32
	if gc.GetCount1TableSelect() {
		tableIndex = 33
	}
	if tableIndex < 0 || tableIndex >= len(common.BaseTables) {
		return 0, fmt.Errorf("unsupported count1 huffman table: %d", tableIndex)
	}
	table := common.BaseTables[tableIndex]
	if len(table.LUT) == 0 {
		return 0, fmt.Errorf("unsupported count1 huffman table: %d", tableIndex)
	}

	var scratch uint32
	if cap(*spectralValues) < 576 {
		*spectralValues = make([]int, 576)
	} else {
		*spectralValues = (*spectralValues)[:576]
	}
	startLine := int(gc.BigValues) * 2
	if startLine > 576 {
		return 0, fmt.Errorf("invalid big_values: %d", gc.BigValues)
	}
	writePos := startLine
	for br.Pos < part23EndBit {
		if writePos >= 576 || writePos+4 > 576 {
			break
		}
		startPos := br.Pos

		v, w, x, y, err := decodeHuffmanQuad(br, &table, part23EndBit, &scratch)
		if err != nil {
			br.Pos = startPos
			if strings.HasPrefix(err.Error(), "huffman data exceeds part23 length") {
				break
			}
			return writePos - startLine, err
		}
		values := [4]int{v, w, x, y}
		copy((*spectralValues)[writePos:], values[:])
		writePos += len(values)
	}

	return writePos - startLine, nil
}
