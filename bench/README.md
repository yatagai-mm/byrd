# Benchmarks

This directory contains MP3 decode speed benchmarks for:

- `byrd`
- `github.com/hajimehoshi/go-mp3`

Inputs are fixed to:

- `static/440hz.mp3`
- `static/alarm.mp3`
- `static/song.mp3`
- `static/synth.mp3`
- `static/circle-reading.mp3`

Each input is loaded into memory before timing starts so file IO does not count. Decoded PCM is written
to `io.Discard`, so the benchmark excludes file I/O and output buffer growth.

Run from the `bench/` directory:

```sh
go test -bench .
```
