package decoder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kota-yata/byrd-mp3/internal/common"
	"github.com/kota-yata/byrd-mp3/internal/header"
	"github.com/kota-yata/byrd-mp3/internal/hybrid"
	"github.com/kota-yata/byrd-mp3/internal/maindata"
	"github.com/kota-yata/byrd-mp3/internal/stereo"
	"github.com/kota-yata/byrd-mp3/internal/synthesis"
)

const GRANULE_COUNT = 2

func OpenMP3File(path string) (io.ReadCloser, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
	default:
		return nil, fmt.Errorf("unsupported file format: %s", path)
	}
	return os.Open(path)
}

type Reader struct {
	r *bufio.Reader

	mainDataReservoir maindata.Reservoir
	sideInfoBuf       []byte
	cur               []byte
	mainData          []byte
	scalefactors      [2][2]maindata.Scalefactors
	count1            [2][2]int
	spectralValues    [2][2][576]int
	requantizedValues [2][2][576]float32
	reorderedValues   [2][2][576]float32
	hybridValues      [2][2][576]float32
	overlapState      [2][32][18]float32
	hybridSamples     [2][2][32][18]float32
	synthesisState    [2]synthesis.PolyphaseState
	pcmSamples        [2][2][576]float32

	pcm        []byte
	pcmPos     int
	sampleRate uint16
	channels   int
	eof        bool
}

func NewReader(r *bufio.Reader) *Reader {
	return &Reader{r: r}
}

func (d *Reader) SampleRate() uint16 {
	return d.sampleRate
}

func (d *Reader) Channels() int {
	return d.channels
}

func (d *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.pcmPos == len(d.pcm) && !d.eof {
		if err := d.decodeFrame(); err != nil {
			if err == io.EOF {
				d.eof = true
				break
			}
			return 0, err
		}
	}
	if d.pcmPos == len(d.pcm) {
		return 0, io.EOF
	}

	n := copy(p, d.pcm[d.pcmPos:])
	d.pcmPos += n
	if d.pcmPos == len(d.pcm) {
		d.pcm = d.pcm[:0]
		d.pcmPos = 0
	}
	return n, nil
}

func DecodeMP3Frames(r *bufio.Reader) ([]int16, uint16, int, error) {
	reader := NewReader(r)
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, 0, err
	}
	out := make([]int16, len(raw)/2)
	for i := range out {
		j := i * 2
		out[i] = int16(uint16(raw[j]) | uint16(raw[j+1])<<8)
	}
	return out, reader.SampleRate(), reader.Channels(), nil
}

func (d *Reader) decodeFrame() error {
	var h header.MP3FrameHeader
	d.pcm = d.pcm[:0]
	d.pcmPos = 0

	h = header.MP3FrameHeader{} // reset frame state
	if err := header.ReadHeader(&h, d.r); err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return fmt.Errorf("failed to read MP3 frame header: %w", err)
	}

	if !h.ValidateCRC(d.r) {
		return fmt.Errorf("CRC check failed for MP3 frame")
	}

	sideInfoLen := header.GetSideInfoLength(&h)
	if cap(d.sideInfoBuf) < sideInfoLen {
		d.sideInfoBuf = make([]byte, sideInfoLen)
	}
	d.sideInfoBuf = d.sideInfoBuf[:sideInfoLen]
	if _, err := io.ReadFull(d.r, d.sideInfoBuf); err != nil {
		return fmt.Errorf("failed to read side info: %w", err)
	}
	sideInfo, err := header.ReadSideInfo(&h, d.sideInfoBuf)
	if err != nil {
		return fmt.Errorf("failed to read side info: %w", err)
	}

	frameLen, err := h.GetFrameLength()
	if err != nil {
		return fmt.Errorf("failed to calculate frame length: %w", err)
	}
	crcLen := 0
	if h.HasCRC() {
		crcLen = 2
	}

	mainDataLen := frameLen - 4 - sideInfoLen - crcLen
	if cap(d.cur) < mainDataLen {
		d.cur = make([]byte, mainDataLen)
	}
	d.cur = d.cur[:mainDataLen]
	if _, err := io.ReadFull(d.r, d.cur); err != nil {
		return fmt.Errorf("failed to read main data: %w", err)
	}
	if err := maindata.ReadMainData(sideInfo.MainDataBegin, &d.mainDataReservoir, d.cur, &d.mainData); err != nil {
		return fmt.Errorf("failed to read main data: %w", err)
	}
	br := common.NewBitReader(d.mainData)
	frameChannels := 2
	if h.GetChannelMode() == header.ChannelModeMono {
		frameChannels = 1
	}
	if d.sampleRate == 0 {
		d.sampleRate = h.GetSampleRate()
		d.channels = frameChannels
	} else if d.sampleRate != h.GetSampleRate() || d.channels != frameChannels {
		return fmt.Errorf("variable stream parameters are not supported: sampleRate=%d/%d channels=%d/%d", d.sampleRate, h.GetSampleRate(), d.channels, frameChannels)
	}

	for gr := range GRANULE_COUNT {
		for ch := 0; ch < frameChannels; ch++ {
			gc := &sideInfo.Granule[gr][ch]
			part23Start := br.Pos
			part23End := part23Start + int(gc.Part23Length)

			var prev *maindata.Scalefactors
			if gr == 1 {
				prev = &d.scalefactors[0][ch]
			}
			_, err = maindata.ParseScaleFactor(br, gc, sideInfo.SCFSI[ch], gr, prev, &d.scalefactors[gr][ch])
			if err != nil {
				return fmt.Errorf("failed to parse scalefactors: frame granule=%d channel=%d err=%w", gr, ch, err)
			}

			huffmanLen := part23End - br.Pos
			if huffmanLen < 0 {
				return fmt.Errorf("main data underrun: frame granule=%d channel=%d part23=%d bits consumed for scalefactors=%d", gr, ch, gc.Part23Length, br.Pos-part23Start)
			}
			spectralBuffer := d.spectralValues[gr][ch][:]
			_, err = maindata.ParseBigValues(br, h.GetSampleRate(), gc, part23End, &spectralBuffer)
			if err != nil {
				return fmt.Errorf("failed to parse big values: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			count1Lines, err := maindata.ParseCount1Values(br, gc, part23End, &spectralBuffer)
			if err != nil {
				return fmt.Errorf("failed to parse count1 values: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			d.count1[gr][ch] = int(gc.BigValues)*2 + count1Lines
			requantizedBuffer := d.requantizedValues[gr][ch][:]
			if err := maindata.Requantize(h.GetSampleRate(), gc, &d.scalefactors[gr][ch], spectralBuffer, &requantizedBuffer); err != nil {
				return fmt.Errorf("failed to requantize values: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			reorderedBuffer := d.reorderedValues[gr][ch][:]
			if err := maindata.Reorder(h.GetSampleRate(), gc, requantizedBuffer, &reorderedBuffer); err != nil {
				return fmt.Errorf("failed to reorder values: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			br.Pos = part23End
			if br.Pos > len(d.mainData)*8 {
				return fmt.Errorf("main data overrun: frame granule=%d channel=%d part23=%d", gr, ch, gc.Part23Length)
			}
		}
		if frameChannels == 2 {
			left := d.reorderedValues[gr][0][:]
			right := d.reorderedValues[gr][1][:]
			if err := stereo.ApplyJointStereo(h.GetSampleRate(), h.GetChannelMode(), h.GetModeExtension(), &sideInfo.Granule[gr][0], &d.scalefactors[gr][0], left, right, d.count1[gr][0], d.count1[gr][1]); err != nil {
				return fmt.Errorf("failed to apply joint stereo: frame granule=%d err=%w", gr, err)
			}
		}
		for ch := 0; ch < frameChannels; ch++ {
			hybridBuffer := d.hybridValues[gr][ch][:]
			copy(hybridBuffer, d.reorderedValues[gr][ch][:])
			if err := hybrid.ApplyAliasReduction(&sideInfo.Granule[gr][ch], hybridBuffer); err != nil {
				return fmt.Errorf("failed to apply alias reduction: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			if err := hybrid.HybridSynthesis(&sideInfo.Granule[gr][ch], hybridBuffer, &d.overlapState[ch], &d.hybridSamples[gr][ch]); err != nil {
				return fmt.Errorf("failed to run hybrid synthesis: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
			synthesis.ApplyFrequencyInversion(&d.hybridSamples[gr][ch])
			if err := synthesis.SynthesizeGranule(&d.hybridSamples[gr][ch], &d.synthesisState[ch], &d.pcmSamples[gr][ch]); err != nil {
				return fmt.Errorf("failed to run polyphase synthesis: frame granule=%d channel=%d err=%w", gr, ch, err)
			}
		}
		if frameChannels == 1 {
			for i := 0; i < 576; i++ {
				d.pcm = appendInt16LE(d.pcm, synthesis.QuantizeSample(d.pcmSamples[gr][0][i]))
			}
		} else {
			for i := 0; i < 576; i++ {
				d.pcm = appendInt16LE(d.pcm, synthesis.QuantizeSample(d.pcmSamples[gr][0][i]))
				d.pcm = appendInt16LE(d.pcm, synthesis.QuantizeSample(d.pcmSamples[gr][1][i]))
			}
		}
	}

	return nil
}

func appendInt16LE(dst []byte, sample int16) []byte {
	u := uint16(sample)
	return append(dst, byte(u), byte(u>>8))
}
