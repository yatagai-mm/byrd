package decoder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yatagai-mm/byrd/internal/common"
	"github.com/yatagai-mm/byrd/internal/header"
	"github.com/yatagai-mm/byrd/internal/hybrid"
	"github.com/yatagai-mm/byrd/internal/maindata"
	"github.com/yatagai-mm/byrd/internal/stereo"
	"github.com/yatagai-mm/byrd/internal/synthesis"
)

// Just to make sure no error occurs when parsing bundled MP3 data.
func TestParseStaticMP3RealData(t *testing.T) {
	paths := listStaticMP3Paths(t)

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			runParseRealDataTest(t, path)
		})
	}
}

func listStaticMP3Paths(t *testing.T) []string {
	t.Helper()

	pattern := filepath.Join("..", "..", "static", "*.mp3")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("failed to list static mp3 files: %v", err)
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		t.Fatalf("no mp3 files found under static/")
	}
	return paths
}

func runParseRealDataTest(t *testing.T, path string) {
	t.Helper()

	f, err := OpenMP3File(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filepath.Base(path), err)
	}
	defer f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", filepath.Base(path), err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory", filepath.Base(path))
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty", filepath.Base(path))
	}

	r := bufio.NewReader(f)
	var mainDataReservoir maindata.Reservoir
	var sideInfoBuf []byte
	var cur []byte
	var mainData []byte
	frameIndex := 0
	fileLabel := filepath.Base(path)
	frameSummary := []string{}
	logFrameSummary := func() {
		if len(frameSummary) == 0 {
			return
		}
		t.Logf("file=%s frame=%d %s", fileLabel, frameIndex, strings.Join(frameSummary, " | "))
	}

	for {
		var h header.MP3FrameHeader
		err = header.ReadHeader(&h, r)
		if err == io.EOF {
			break
		}
		if err != nil {
			if frameIndex > 0 {
				t.Logf("file=%s stopping after %d parsed frames: %v", fileLabel, frameIndex, err)
				break
			}
			t.Fatalf("file=%s frame=%d: failed to read header: %v", fileLabel, frameIndex, err)
		}

		bitrateKbps, free := h.GetBitrateKbps()
		if free {
			t.Fatalf("file=%s frame=%d: free bitrate is not supported", fileLabel, frameIndex)
		}
		frameLen, err := h.GetFrameLength()
		if err != nil {
			t.Fatalf("file=%s frame=%d: failed to get frame length: %v", fileLabel, frameIndex, err)
		}
		if !h.ValidateCRC(r) {
			t.Fatalf("file=%s frame=%d: CRC validation failed", fileLabel, frameIndex)
		}

		sideInfoLen := header.GetSideInfoLength(&h)
		if cap(sideInfoBuf) < sideInfoLen {
			sideInfoBuf = make([]byte, sideInfoLen)
		}
		sideInfoBuf = sideInfoBuf[:sideInfoLen]
		_, err = io.ReadFull(r, sideInfoBuf)
		if err != nil {
			t.Fatalf("file=%s frame=%d: failed to read side info: %v", fileLabel, frameIndex, err)
		}
		sideInfo, err := header.ReadSideInfo(&h, sideInfoBuf)
		if err != nil {
			t.Fatalf("file=%s frame=%d: failed to read side info: %v", fileLabel, frameIndex, err)
		}

		crcLen := 0
		if h.HasCRC() {
			crcLen = 2
		}
		mainDataLen := frameLen - 4 - sideInfoLen - crcLen
		if mainDataLen < 0 {
			t.Fatalf("file=%s frame=%d: invalid main data length %d", fileLabel, frameIndex, mainDataLen)
		}

		if cap(cur) < mainDataLen {
			cur = make([]byte, mainDataLen)
		}
		cur = cur[:mainDataLen]
		_, err = io.ReadFull(r, cur)
		if err != nil {
			t.Fatalf("file=%s frame=%d: failed to read current frame main data: %v", fileLabel, frameIndex, err)
		}
		err = maindata.ReadMainData(sideInfo.MainDataBegin, &mainDataReservoir, cur, &mainData)
		if err != nil {
			t.Fatalf("file=%s frame=%d: failed to reconstruct main data: %v", fileLabel, frameIndex, err)
		}

		channels := 2
		if h.GetChannelMode() == header.ChannelModeMono {
			channels = 1
		}
		frameSummary = []string{
			fmt.Sprintf(
				"bitrate=%dkbps sampleRate=%d padding=%v hasCRC=%v channelMode=%s modeExt=%d copyright=%v original=%v emphasis=%d frameLen=%d sideInfoLen=%d mainDataBegin=%d mainDataLen=%d reservoirLen=%d",
				bitrateKbps,
				h.GetSampleRate(),
				h.Padding(),
				h.HasCRC(),
				h.GetChannelMode(),
				h.GetModeExtension(),
				h.IsCopyrighted(),
				h.IsOriginal(),
				h.GetEmphasis(),
				frameLen,
				sideInfoLen,
				sideInfo.MainDataBegin,
				mainDataLen,
				mainDataReservoir.Len(),
			),
		}
		for ch := 0; ch < channels; ch++ {
			frameSummary = append(frameSummary, fmt.Sprintf("ch=%d scfsi=%v", ch, sideInfo.SCFSI[ch]))
		}

		br := common.NewBitReader(mainData)
		var prev [2]maindata.Scalefactors
		var spectralValues [2][576]int
		var requantizedValues [2][576]float32
		var reorderedValues [2][576]float32
		var stereoValues [2][576]float32
		var hybridValues [2][576]float32
		var overlapState [2][32][18]float32
		var hybridSamples [2][32][18]float32
		var synthesisState [2]synthesis.PolyphaseState
		var pcmSamples [2][576]float32
		for gr := 0; gr < 2; gr++ {
			var granuleScalefactors [2]maindata.Scalefactors
			var granuleCount1 [2]int
			for ch := 0; ch < channels; ch++ {
				gc := &sideInfo.Granule[gr][ch]
				part23Start := br.Pos
				part23End := part23Start + int(gc.Part23Length)
				var scalefactors maindata.Scalefactors
				var prevPtr *maindata.Scalefactors
				if gr == 1 {
					prevPtr = &prev[ch]
				}

				part2Bits, err := maindata.ParseScaleFactor(br, gc, sideInfo.SCFSI[ch], gr, prevPtr, &scalefactors)
				if err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to parse scalefactors: %v", fileLabel, frameIndex, gr, ch, err)
				}
				prev[ch] = scalefactors
				granuleScalefactors[ch] = scalefactors

				spectralBuffer := spectralValues[ch][:]
				bigValueLines, err := maindata.ParseBigValues(br, h.GetSampleRate(), gc, part23End, &spectralBuffer)
				if err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to parse big values: %v", fileLabel, frameIndex, gr, ch, err)
				}
				count1Lines, err := maindata.ParseCount1Values(br, gc, part23End, &spectralBuffer)
				if err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to parse count1 values: %v", fileLabel, frameIndex, gr, ch, err)
				}
				granuleCount1[ch] = int(gc.BigValues)*2 + count1Lines
				requantizedBuffer := requantizedValues[ch][:]
				if err := maindata.Requantize(h.GetSampleRate(), gc, &scalefactors, spectralBuffer, &requantizedBuffer); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to requantize values: %v", fileLabel, frameIndex, gr, ch, err)
				}
				reorderedBuffer := reorderedValues[ch][:]
				if err := maindata.Reorder(h.GetSampleRate(), gc, requantizedBuffer, &reorderedBuffer); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to reorder values: %v", fileLabel, frameIndex, gr, ch, err)
				}
				nonZeroRequantized := 0
				for _, v := range requantizedBuffer {
					if v != 0 {
						nonZeroRequantized++
					}
				}
				nonZeroReordered := 0
				for _, v := range reorderedBuffer {
					if v != 0 {
						nonZeroReordered++
					}
				}
				frameSummary = append(frameSummary, fmt.Sprintf(
					"gr=%d ch=%d part23=%d part2=%d part3=%d bigValues=%d bigValueLines=%d count1Lines=%d globalGain=%d scalefacCompress=%d tableSelect=%v subblockGain=%v region0=%d region1=%d windowSwitching=%v blockType=%s mixed=%v preflag=%v scalefacScale=%v count1Table=%v long=%v short=%v spectralLines=%d requantizedNonZero=%d reorderedNonZero=%d",
					gr,
					ch,
					gc.Part23Length,
					part2Bits,
					int(gc.Part23Length)-part2Bits,
					gc.BigValues,
					bigValueLines,
					count1Lines,
					gc.GlobalGain,
					gc.ScalefacCompress,
					gc.TableSelect,
					gc.SubblockGain,
					gc.Region0Count,
					gc.Region1Count,
					gc.GetWindowSwitching(),
					gc.GetBlockType(),
					gc.GetMixedBlockFlag(),
					gc.GetPreflag(),
					gc.GetScalefacScale(),
					gc.GetCount1TableSelect(),
					scalefactors.Long,
					scalefactors.Short,
					576,
					nonZeroRequantized,
					nonZeroReordered,
				))

				br.Pos = part23End
				if br.Pos > len(mainData)*8 {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: part23 overruns main data bitstream", fileLabel, frameIndex, gr, ch)
				}
			}
			for ch := 0; ch < channels; ch++ {
				copy(stereoValues[ch][:], reorderedValues[ch][:])
			}
			if channels == 2 {
				left := stereoValues[0][:]
				right := stereoValues[1][:]
				if err := stereo.ApplyJointStereo(h.GetSampleRate(), h.GetChannelMode(), h.GetModeExtension(), &sideInfo.Granule[gr][0], &granuleScalefactors[0], left, right, granuleCount1[0], granuleCount1[1]); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d: failed to apply joint stereo: %v", fileLabel, frameIndex, gr, err)
				}
				for ch := 0; ch < channels; ch++ {
					nonZeroStereo := 0
					for _, v := range stereoValues[ch] {
						if v != 0 {
							nonZeroStereo++
						}
					}
					frameSummary = append(frameSummary, fmt.Sprintf("gr=%d ch=%d stereoNonZero=%d", gr, ch, nonZeroStereo))
				}
			}
			for ch := 0; ch < channels; ch++ {
				copy(hybridValues[ch][:], stereoValues[ch][:])
				if err := hybrid.ApplyAliasReduction(&sideInfo.Granule[gr][ch], hybridValues[ch][:]); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to apply alias reduction: %v", fileLabel, frameIndex, gr, ch, err)
				}
				nonZeroHybrid := 0
				for _, v := range hybridValues[ch] {
					if v != 0 {
						nonZeroHybrid++
					}
				}
				if err := hybrid.HybridSynthesis(&sideInfo.Granule[gr][ch], hybridValues[ch][:], &overlapState[ch], &hybridSamples[ch]); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to run hybrid synthesis: %v", fileLabel, frameIndex, gr, ch, err)
				}
				nonZeroSamples := 0
				for sb := range hybridSamples[ch] {
					for i := range hybridSamples[ch][sb] {
						if hybridSamples[ch][sb][i] != 0 {
							nonZeroSamples++
						}
					}
				}
				synthesis.ApplyFrequencyInversion(&hybridSamples[ch])
				nonZeroInverted := 0
				for sb := range hybridSamples[ch] {
					for i := range hybridSamples[ch][sb] {
						if hybridSamples[ch][sb][i] != 0 {
							nonZeroInverted++
						}
					}
				}
				if err := synthesis.SynthesizeGranule(&hybridSamples[ch], &synthesisState[ch], &pcmSamples[ch]); err != nil {
					logFrameSummary()
					t.Fatalf("file=%s frame=%d gr=%d ch=%d: failed to run polyphase synthesis: %v", fileLabel, frameIndex, gr, ch, err)
				}
				nonZeroPCM := 0
				for _, v := range pcmSamples[ch] {
					if v != 0 {
						nonZeroPCM++
					}
				}
				frameSummary = append(frameSummary, fmt.Sprintf("gr=%d ch=%d aliasNonZero=%d hybridNonZero=%d invertedNonZero=%d pcmNonZero=%d", gr, ch, nonZeroHybrid, nonZeroSamples, nonZeroInverted, nonZeroPCM))
			}
		}
		frameIndex++
	}

	if frameIndex == 0 {
		t.Fatalf("no frames parsed from %s", fileLabel)
	}
}
