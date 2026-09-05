package maindata

import (
	"fmt"

	"github.com/yatagai-mm/byrd/internal/common"
)

func selectTable(sampleRate uint16, gc *common.GranuleChannelInfo, spectralLineIndex int) (*common.HuffmanTable, error) {
	if gc == nil {
		return nil, fmt.Errorf("nil granule channel info")
	}
	if spectralLineIndex < 0 {
		return nil, fmt.Errorf("invalid spectral line index: %d", spectralLineIndex)
	}

	sfBands, ok := common.SCALEFACTOR_BAND_INDICES[sampleRate]
	if !ok {
		return nil, fmt.Errorf("unsupported sample rate for scalefactor bands: %d", sampleRate)
	}

	var tableIndex int
	if !gc.GetWindowSwitching() || gc.GetBlockType() != common.BlockTypeShort {
		region1StartSFB := int(gc.Region0Count) + 1
		region2StartSFB := int(gc.Region0Count) + int(gc.Region1Count) + 2
		if region1StartSFB >= len(sfBands.Long) {
			region1StartSFB = len(sfBands.Long) - 1
		}
		if region2StartSFB >= len(sfBands.Long) {
			region2StartSFB = len(sfBands.Long) - 1
		}
		region1Start := sfBands.Long[region1StartSFB]
		region2Start := sfBands.Long[region2StartSFB]
		if spectralLineIndex < region1Start {
			tableIndex = int(gc.TableSelect[0])
		} else if spectralLineIndex < region2Start {
			tableIndex = int(gc.TableSelect[1])
		} else {
			tableIndex = int(gc.TableSelect[2])
		}
	} else {
		// Short-block Huffman regions are used only for pure/mixed short blocks.
		// Start/end blocks keep long-block region boundaries even though
		// window_switching is set.
		region1Start := sfBands.Short[3] * SCALEFACTOR_SHORT_WINDOW_COUNT
		if gc.GetMixedBlockFlag() {
			region1Start = sfBands.Long[8]
		}
		if spectralLineIndex < region1Start {
			tableIndex = int(gc.TableSelect[0])
		} else {
			tableIndex = int(gc.TableSelect[1])
		}
	}

	if tableIndex == 0 {
		return &common.BaseTables[0], nil
	}

	if tableIndex < 0 || tableIndex >= len(common.BaseTables) {
		return nil, fmt.Errorf("unknown huffman table: %d", tableIndex)
	}
	table := &common.BaseTables[tableIndex]
	if len(table.LUT) == 0 {
		return nil, fmt.Errorf("unsupported huffman table: %d", tableIndex)
	}
	return table, nil
}

func guardedReadBit(br *common.BitReader, limit int, scratch *uint32) error {
	if br.Pos+1 > limit {
		return fmt.Errorf("huffman data exceeds part23 length: need 1 more bit, have %d", limit-br.Pos)
	}
	return br.ReadBitsTo(scratch, 1)
}

func guardedReadBits(br *common.BitReader, limit int, n int, scratch *uint32) error {
	if br.Pos+n > limit {
		return fmt.Errorf("huffman data exceeds part23 length: need %d more bits, have %d", n, limit-br.Pos)
	}
	return br.ReadBitsTo(scratch, n)
}

func huffmanDataLimitError(limit int, pos int) error {
	return fmt.Errorf("huffman data exceeds part23 length: incomplete code, have %d bits", max(0, limit-pos))
}

func decodeHuffmanSymbolLUT(br *common.BitReader, table *common.HuffmanTable, limit int, scratch *uint32) (uint16, error) {
	if table == nil {
		return 0, fmt.Errorf("nil huffman table")
	}
	if table.IsZero {
		return 0, nil
	}
	if len(table.LUT) == 0 {
		return 0, fmt.Errorf("empty huffman table")
	}
	if table.LUTBits == 0 {
		return 0, fmt.Errorf("missing huffman LUT table")
	}

	// Each level consumes at most seven bits. MP3's longest 19-bit codeword
	// therefore needs no more than three dependent table lookups.
	offset := 0
	width := int(table.LUTBits)
	for level := 0; level < 8; level++ {
		available := limit - br.Pos
		if available <= 0 {
			return 0, huffmanDataLimitError(limit, br.Pos)
		}
		readWidth := min(width, available)
		if err := br.ReadBitsTo(scratch, readWidth); err != nil {
			return 0, err
		}
		prefix := int(*scratch)
		if readWidth < width {
			prefix <<= width - readWidth
		}

		entryIndex := offset + prefix
		if entryIndex < 0 || entryIndex >= len(table.LUT) {
			return 0, fmt.Errorf("invalid huffman LUT index")
		}
		entry := table.LUT[entryIndex]
		if entry == 0 {
			if readWidth < width {
				return 0, huffmanDataLimitError(limit, br.Pos)
			}
			return 0, fmt.Errorf("invalid huffman LUT prefix")
		}

		bits := int(entry & 0xff)
		if bits != 0 {
			if bits > readWidth {
				return 0, huffmanDataLimitError(limit, br.Pos)
			}
			br.Pos -= readWidth - bits
			return uint16((entry >> 8) & 0xff), nil
		}

		if readWidth < width {
			return 0, huffmanDataLimitError(limit, br.Pos)
		}
		offset = int(entry >> 16)
		width = int((entry >> 8) & 0xff)
		if width == 0 {
			return 0, fmt.Errorf("invalid huffman LUT subtable")
		}
	}

	return 0, fmt.Errorf("huffman LUT depth exceeds limit")
}

func decodeHuffmanPair(br *common.BitReader, table *common.HuffmanTable, limit int, scratch *uint32) (int, int, error) {
	node, err := decodeHuffmanSymbolLUT(br, table, limit, scratch)
	if err != nil {
		return 0, 0, err
	}
	return decodeHuffmanPairLeaf(br, table, node, limit, scratch)
}

func decodeHuffmanPairLeaf(br *common.BitReader, table *common.HuffmanTable, node uint16, limit int, scratch *uint32) (int, int, error) {
	x := int((node >> 4) & 0xF)
	y := int(node & 0xF)
	if table.Linbits > 0 {
		if x == 15 {
			if err := guardedReadBits(br, limit, table.Linbits, scratch); err != nil {
				return 0, 0, err
			}
			x += int(*scratch)
		}
		if x != 0 {
			if err := guardedReadBit(br, limit, scratch); err != nil {
				return 0, 0, err
			}
			if *scratch == 1 {
				x = -x
			}
		}
		if y == 15 {
			if err := guardedReadBits(br, limit, table.Linbits, scratch); err != nil {
				return 0, 0, err
			}
			y += int(*scratch)
		}
		if y != 0 {
			if err := guardedReadBit(br, limit, scratch); err != nil {
				return 0, 0, err
			}
			if *scratch == 1 {
				y = -y
			}
		}
		return x, y, nil
	}
	if x != 0 {
		if err := guardedReadBit(br, limit, scratch); err != nil {
			return 0, 0, err
		}
		if *scratch == 1 {
			x = -x
		}
	}
	if y != 0 {
		if err := guardedReadBit(br, limit, scratch); err != nil {
			return 0, 0, err
		}
		if *scratch == 1 {
			y = -y
		}
	}
	return x, y, nil
}

func decodeHuffmanQuad(br *common.BitReader, table *common.HuffmanTable, limit int, scratch *uint32) (int, int, int, int, error) {
	node, err := decodeHuffmanSymbolLUT(br, table, limit, scratch)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return decodeHuffmanQuadLeaf(br, node, limit, scratch)
}

func decodeHuffmanQuadLeaf(br *common.BitReader, node uint16, limit int, scratch *uint32) (int, int, int, int, error) {
	v := int((node >> 3) & 0x1)
	w := int((node >> 2) & 0x1)
	x := int((node >> 1) & 0x1)
	y := int(node & 0x1)
	values := []*int{&v, &w, &x, &y}
	for _, value := range values {
		if *value == 0 {
			continue
		}
		if err := guardedReadBit(br, limit, scratch); err != nil {
			return 0, 0, 0, 0, err
		}
		if *scratch == 1 {
			*value = -*value
		}
	}
	return v, w, x, y, nil
}
