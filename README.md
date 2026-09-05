![Gopher with Byrd letter](./static/byrd-gopher.png)
## Byrd
MP3 (MPEG-1 Layer 3) decoder in Go. No dependency to third-party libraries.

Byrd reduces alloc count per operation by reusing memory addresses from previous frame as much as possible.

### Usage
```bash
go get github.com/kota-yata/byrd-mp3
```
```go
import byrd "github.com/kota-yata/byrd-mp3"
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

### Benchmark result as of v0.2.0 with go-mp3

```
goos: darwin
goarch: arm64
pkg: byrd-bench
cpu: Apple M5 Pro
BenchmarkDecode/byrd/440hz-18         	       3	 417061153 ns/op	   1.92 MB/s	 2171269 B/op	   13478 allocs/op
BenchmarkDecode/go-mp3/440hz-18       	       2	 637946770 ns/op	   1.25 MB/s	330494200 B/op	  832075 allocs/op
BenchmarkDecode/byrd/alarm-18         	       7	 155915506 ns/op	   4.71 MB/s	 1851371 B/op	    4495 allocs/op
BenchmarkDecode/go-mp3/alarm-18       	       5	 208990642 ns/op	   3.51 MB/s	91337355 B/op	  225258 allocs/op
BenchmarkDecode/byrd/song-18          	       2	 608771146 ns/op	   6.81 MB/s	 7642208 B/op	   14887 allocs/op
BenchmarkDecode/go-mp3/song-18        	       2	 742717458 ns/op	   5.58 MB/s	285375552 B/op	  693321 allocs/op
BenchmarkDecode/byrd/synth-18         	      16	  70732844 ns/op	   4.35 MB/s	  665817 B/op	    1875 allocs/op
BenchmarkDecode/go-mp3/synth-18       	      12	  93210142 ns/op	   3.30 MB/s	41535624 B/op	  102850 allocs/op
BenchmarkDecode/byrd/circle-reading-18         	       1	21746207083 ns/op	   3.91 MB/s	216393248 B/op	  618327 allocs/op
BenchmarkDecode/go-mp3/circle-reading-18       	       1	29533760500 ns/op	   2.88 MB/s	12753789016 B/op	31593997 allocs/op
```

```
goos: linux
goarch: amd64
pkg: byrd-bench
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkDecode/byrd/440hz-2                   1        1323001370 ns/op    0.60 MB/s     2168944 B/op      13479 allocs/op
BenchmarkDecode/go-mp3/440hz-2                 1        2271381282 ns/op    0.35 MB/s    330489872 B/op    832058 allocs/op
BenchmarkDecode/byrd/alarm-2                   2         627322754 ns/op    1.17 MB/s     1849712 B/op       4496 allocs/op
BenchmarkDecode/go-mp3/alarm-2                 2         696010355 ns/op    1.05 MB/s    91336840 B/op     225257 allocs/op
BenchmarkDecode/byrd/song-2                    1        2268860725 ns/op    1.83 MB/s     7639776 B/op      14887 allocs/op
BenchmarkDecode/go-mp3/song-2                  1        2565913466 ns/op    1.61 MB/s    285370104 B/op    693316 allocs/op
BenchmarkDecode/byrd/synth-2                   4         279689512 ns/op    1.10 MB/s      665382 B/op       1875 allocs/op
BenchmarkDecode/go-mp3/synth-2                 3         333685004 ns/op    0.92 MB/s    41532210 B/op     102849 allocs/op
BenchmarkDecode/byrd/circle-reading-2                  1        75109323119 ns/op          1.13 MB/s    216390816 B/op    618327 allocs/op
BenchmarkDecode/go-mp3/circle-reading-2                1        93953570127 ns/op          0.91 MB/s    12753775648 B/op        31593989 allocs/op
```
