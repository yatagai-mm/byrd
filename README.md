![Gopher with Byrd letter](./static/byrd-gopher.png)
## Byrd
MP3 (MPEG-1 Layer 3) decoder in Go. No dependency to third-party libraries.

Byrd allocates very less amount of memory per operation by reusing memory space as much as possible among all frames. See the benchmark result below for further details.

### Usage
```bash
go get github.com/yatagai-mm/byrd
```
```go
import byrd "github.com/yatagai-mm/byrd"
```

```go
f, err := os.Open("input.mp3")
if err != nil {
	log.Fatal(err)
}
defer f.Close()

dec, err := byrd.NewDecoder(f)
if err != nil {
	log.Fatal(err)
}

pcm, err := io.ReadAll(dec)
if err != nil {
	log.Fatal(err)
}

log.Printf("decoded %d PCM bytes at %d Hz with %d channels", len(pcm), dec.SampleRate(), dec.Channels())
```

Decode MP3 file at once (non-streaming use cases)

```go
pcmData, err := dec.BatchDecode()
if err != nil {
	log.Fatal(err)
}

if err := pcmData.WriteWAVFile("output.wav"); err != nil {
	log.Fatal(err)
}
```

See examples under example/ for further usage.

### Benchmark result as of v0.2.2 with go-mp3
hajimehoshi/go-mp3 is a widely used mp3 decoder in Go. Below is the comparison between Byrd and go-mp3 using `go bench`. The benchmark code is at bench/ directory.

```
goos: darwin
goarch: arm64
pkg: byrd-bench
cpu: Apple M5 Pro
BenchmarkDecode/byrd/440hz-18         	      10	 204057212 ns/op	   3.91 MB/s	  778300 B/op	   11978 allocs/op
BenchmarkDecode/go-mp3/440hz-18       	       4	 652273021 ns/op	   1.22 MB/s	330497584 B/op	  832078 allocs/op
BenchmarkDecode/byrd/alarm-18         	      27	  85147840 ns/op	   8.62 MB/s	  307979 B/op	    3266 allocs/op
BenchmarkDecode/go-mp3/alarm-18       	      10	 216952650 ns/op	   3.38 MB/s	91338072 B/op	  225259 allocs/op
BenchmarkDecode/byrd/song-18          	       6	 337278694 ns/op	  12.29 MB/s	  668049 B/op	    9933 allocs/op
BenchmarkDecode/go-mp3/song-18        	       3	 773389000 ns/op	   5.36 MB/s	285375106 B/op	  693318 allocs/op
BenchmarkDecode/byrd/synth-18         	      61	  38471383 ns/op	   7.99 MB/s	  195112 B/op	    1506 allocs/op
BenchmarkDecode/go-mp3/synth-18       	      24	  96969066 ns/op	   3.17 MB/s	41536080 B/op	  102851 allocs/op
BenchmarkDecode/byrd/circle-reading-18         	       1	11725733917 ns/op	   7.25 MB/s	25442296 B/op	  452201 allocs/op
BenchmarkDecode/go-mp3/circle-reading-18       	       1	29404069208 ns/op	   2.89 MB/s	12753788200 B/op	31593990 allocs/op
```

The macOS results above are medians of three runs. Compared with go-mp3:

| Test case | Byrd speedup | Allocation bytes reduction |
| --- | ---: | ---: |
| 440hz | 219.65% faster | 99.76% less |
| alarm | 154.80% faster | 99.66% less |
| song | 129.30% faster | 99.77% less |
| synth | 152.06% faster | 99.53% less |
| circle-reading | 150.77% faster | 99.80% less |

```
goos: linux
goarch: amd64
pkg: byrd-bench
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkDecode/byrd/440hz-2                   1        1614872297 ns/op           0.49 MB/s      777544 B/op      11979 allocs/op
BenchmarkDecode/go-mp3/440hz-2                 1        1973333114 ns/op           0.40 MB/s    330490256 B/op    832063 allocs/op
BenchmarkDecode/byrd/alarm-2                   2         536960296 ns/op           1.37 MB/s      307680 B/op       3266 allocs/op
BenchmarkDecode/go-mp3/alarm-2                 2         799517662 ns/op           0.92 MB/s    91332664 B/op     225256 allocs/op
BenchmarkDecode/byrd/song-2                    1        2361187268 ns/op           1.75 MB/s      667608 B/op       9933 allocs/op
BenchmarkDecode/go-mp3/song-2                  1        2593420668 ns/op           1.60 MB/s    285361824 B/op    693314 allocs/op
BenchmarkDecode/byrd/synth-2                   4         253898960 ns/op           1.21 MB/s      194800 B/op       1506 allocs/op
BenchmarkDecode/go-mp3/synth-2                 4         285476790 ns/op           1.08 MB/s    41532200 B/op     102849 allocs/op
BenchmarkDecode/byrd/circle-reading-2                  1        75725005457 ns/op          1.12 MB/s    25442560 B/op     452203 allocs/op
BenchmarkDecode/go-mp3/circle-reading-2                1        89675195814 ns/op          0.95 MB/s    12753783256 B/op        31593985 allocs/op
```
