package bench

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	byrd "github.com/yatagai-mm/byrd"

	refmp3 "github.com/hajimehoshi/go-mp3"
)

type decodeResult struct {
	decodedBytes int
	samples      int
}

type decoderFunc func([]byte) (decodeResult, error)

var sink decodeResult

func BenchmarkDecode(b *testing.B) {
	files := []string{
		filepath.Join("..", "static", "440hz.mp3"),
		filepath.Join("..", "static", "alarm.mp3"),
		filepath.Join("..", "static", "song.mp3"),
		filepath.Join("..", "static", "synth.mp3"),
		filepath.Join("..", "static", "circle-reading.mp3"),
	}
	decoders := []struct {
		name string
		fn   decoderFunc
	}{
		{name: "byrd", fn: decodeWithByrd},
		{name: "go-mp3", fn: decodeWithGoMP3},
	}

	for _, path := range files {
		encoded, err := os.ReadFile(path)
		if err != nil {
			b.Fatalf("failed to read %s: %v", path, err)
		}
		if len(encoded) == 0 {
			b.Fatalf("benchmark input is empty: %s", path)
		}

		base := trimExt(filepath.Base(path))
		for _, dec := range decoders {
			b.Run(dec.name+"/"+base, func(b *testing.B) {
				warm, err := dec.fn(encoded)
				if err != nil {
					b.Fatalf("warmup decode failed: %v", err)
				}
				if warm.decodedBytes == 0 || warm.samples == 0 {
					b.Fatalf("decoder returned empty output: %+v", warm)
				}

				b.ReportAllocs()
				b.SetBytes(int64(len(encoded)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					res, err := dec.fn(encoded)
					if err != nil {
						b.Fatalf("decode failed: %v", err)
					}
					sink = res
				}
			})
		}
	}
}

func decodeWithByrd(encoded []byte) (decodeResult, error) {
	dec, err := byrd.NewDecoder(bytes.NewReader(encoded))
	if err != nil {
		return decodeResult{}, err
	}

	decodedBytes, err := io.Copy(io.Discard, dec)
	if err != nil {
		return decodeResult{}, err
	}
	return decodeResult{
		decodedBytes: int(decodedBytes),
		samples:      int(decodedBytes) / 2,
	}, nil
}

func decodeWithGoMP3(encoded []byte) (decodeResult, error) {
	dec, err := refmp3.NewDecoder(bytes.NewReader(encoded))
	if err != nil {
		return decodeResult{}, err
	}
	decodedBytes, err := io.Copy(io.Discard, dec)
	if err != nil {
		return decodeResult{}, err
	}
	return decodeResult{
		decodedBytes: int(decodedBytes),
		samples:      int(decodedBytes) / 2,
	}, nil
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return name[:len(name)-len(ext)]
}
