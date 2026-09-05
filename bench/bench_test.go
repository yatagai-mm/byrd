package bench

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	byrd "github.com/kota-yata/byrd-mp3"

	refmp3 "github.com/hajimehoshi/go-mp3"
)

type decodeResult struct {
	decodedBytes int
	samples      int
}

type decoderFunc func(string) (decodeResult, error)

var sink decodeResult

type testLogger interface {
	Helper()
	Fatalf(format string, args ...any)
}

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
		base := trimExt(filepath.Base(path))
		for _, dec := range decoders {
			b.Run(dec.name+"/"+base, func(b *testing.B) {
				size := mustStatBenchmarkFile(b, path)
				warm, err := dec.fn(path)
				if err != nil {
					b.Fatalf("warmup decode failed: %v", err)
				}
				if warm.decodedBytes == 0 || warm.samples == 0 {
					b.Fatalf("decoder returned empty output: %+v", warm)
				}

				b.ReportAllocs()
				b.SetBytes(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					res, err := dec.fn(path)
					if err != nil {
						b.Fatalf("decode failed: %v", err)
					}
					sink = res
				}
			})
		}
	}
}

func mustStatBenchmarkFile(t testLogger, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("benchmark input is empty: %s", path)
	}
	return info.Size()
}

func decodeWithByrd(path string) (decodeResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return decodeResult{}, err
	}
	defer f.Close()

	dec, err := byrd.NewDecoder(f)
	if err != nil {
		return decodeResult{}, err
	}

	raw, err := io.ReadAll(dec)
	if err != nil {
		return decodeResult{}, err
	}
	sampleCount := len(raw) / 2
	return decodeResult{
		decodedBytes: len(raw),
		samples:      sampleCount,
	}, nil
}

func decodeWithGoMP3(path string) (decodeResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return decodeResult{}, err
	}
	defer f.Close()

	dec, err := refmp3.NewDecoder(f)
	if err != nil {
		return decodeResult{}, err
	}
	raw, err := io.ReadAll(dec)
	if err != nil {
		return decodeResult{}, err
	}
	sampleCount := len(raw) / 2
	return decodeResult{
		decodedBytes: len(raw),
		samples:      sampleCount,
	}, nil
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return name[:len(name)-len(ext)]
}
