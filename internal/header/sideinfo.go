package header

import (
	"fmt"

	"github.com/yatagai-mm/byrd/internal/common"
)

type SideInfo = common.SideInfo
type GranuleChannelInfo = common.GranuleChannelInfo
type BlockType = common.BlockType

const (
	BlockTypeLong  = common.BlockTypeLong
	BlockTypeShort = common.BlockTypeShort

	PURE_SHORT_REGION0_COUNT  = common.PURE_SHORT_REGION0_COUNT
	PURE_SHORT_REGION1_COUNT  = common.PURE_SHORT_REGION1_COUNT
	MIXED_BLOCK_REGION0_COUNT = common.MIXED_BLOCK_REGION0_COUNT
	MIXED_BLOCK_REGION1_COUNT = common.MIXED_BLOCK_REGION1_COUNT
)

func GetSideInfoLength(h *MP3FrameHeader) int {
	if h.GetChannelMode() == ChannelModeMono {
		return 17
	}
	return 32
}

func ReadSideInfo(h *MP3FrameHeader, buf []byte) (*SideInfo, error) {
	if h.HasCRC() {
		h.crcTarget = append(h.crcTarget, buf...)
	}

	br := common.NewBitReader(buf)
	si := &SideInfo{}

	v, err := br.ReadBits(9)
	if err != nil {
		return nil, err
	}
	si.MainDataBegin = uint16(v)

	channels := 2
	if h.GetChannelMode() == ChannelModeMono {
		channels = 1
		_, err = br.ReadBits(5) // read and ignore private_bits
	} else {
		_, err = br.ReadBits(3) // read and ignore private_bits
	}
	if err != nil {
		return nil, err
	}

	for ch := range channels {
		for band := range 4 {
			v, err = br.ReadBits(1)
			if err != nil {
				return nil, err
			}
			si.SCFSI[ch][band] = byte(v)
		}
	}

	for gr := range 2 {
		for ch := range channels {
			gc := &si.Granule[gr][ch]

			v, err = br.ReadBits(12)
			if err != nil {
				return nil, err
			}
			gc.Part23Length = uint16(v)

			v, err = br.ReadBits(9)
			if err != nil {
				return nil, err
			}
			gc.BigValues = uint16(v)

			v, err = br.ReadBits(8)
			if err != nil {
				return nil, err
			}
			gc.GlobalGain = byte(v)

			v, err = br.ReadBits(4)
			if err != nil {
				return nil, err
			}
			gc.ScalefacCompress = byte(v)

			v, err = br.ReadBits(1)
			if err != nil {
				return nil, err
			}
			gc.SetWindowSwitching(v == 1)

			if gc.GetWindowSwitching() {
				v, err = br.ReadBits(2)
				if err != nil {
					return nil, err
				}
				gc.SetBlockType(BlockType(v))

				v, err = br.ReadBits(1)
				if err != nil {
					return nil, err
				}
				gc.SetMixedBlockFlag(v == 1)

				for i := 0; i < 2; i++ {
					v, err = br.ReadBits(5)
					if err != nil {
						return nil, err
					}
					gc.TableSelect[i] = byte(v)
				}
				gc.TableSelect[2] = 0

				for i := 0; i < 3; i++ {
					v, err = br.ReadBits(3)
					if err != nil {
						return nil, err
					}
					gc.SubblockGain[i] = byte(v)
				}

				// window_switching==1 means block is either short or mixed, so long block here is invalid
				if gc.GetBlockType() == BlockTypeLong {
					return nil, fmt.Errorf("invalid side info: block_type=0 with window_switching=1")
				}

				if gc.GetBlockType() == BlockTypeShort && !gc.GetMixedBlockFlag() { // pure short block
					gc.Region0Count = PURE_SHORT_REGION0_COUNT
					gc.Region1Count = PURE_SHORT_REGION1_COUNT
				} else {
					gc.Region0Count = MIXED_BLOCK_REGION0_COUNT
					gc.Region1Count = MIXED_BLOCK_REGION1_COUNT
				}
			} else {
				// if window_switching==0 block_type, mixed_block_flag and subblock_gain are not present as they are fixed to 0
				gc.SetBlockType(BlockTypeLong)
				gc.SetMixedBlockFlag(false)
				gc.SubblockGain = [3]byte{}

				for i := range 3 {
					v, err = br.ReadBits(5)
					if err != nil {
						return nil, err
					}
					gc.TableSelect[i] = byte(v)
				}

				v, err = br.ReadBits(4)
				if err != nil {
					return nil, err
				}
				gc.Region0Count = byte(v)

				v, err = br.ReadBits(3)
				if err != nil {
					return nil, err
				}
				gc.Region1Count = byte(v)
			}

			v, err = br.ReadBits(1)
			if err != nil {
				return nil, err
			}
			gc.SetPreflag(v == 1)

			v, err = br.ReadBits(1)
			if err != nil {
				return nil, err
			}
			gc.SetScalefacScale(v == 1)

			v, err = br.ReadBits(1)
			if err != nil {
				return nil, err
			}
			gc.SetCount1TableSelect(v == 1)
		}
	}

	return si, nil
}
