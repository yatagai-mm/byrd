# Benchmarks

This directory contains MP3 decode speed benchmarks for:

- `byrd`
- `github.com/hajimehoshi/go-mp3`

Inputs are fixed to:

- `static/440hz.mp3`
- `static/alarm.mp3`
- `static/song.mp3`
- `static/synth.mp3`

Run from the `bench/` directory:

```sh
go test -bench .
```
