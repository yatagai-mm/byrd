package main

import (
	"fmt"
	"io"
	"log"
	"os"

	byrd "github.com/yatagai-mm/byrd"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: go run ./example/stream_pcm <input.mp3>")
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	dec, err := byrd.NewDecoder(f)
	if err != nil {
		log.Fatal(err)
	}

	raw, err := io.ReadAll(dec)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("sample_rate=%d\n", dec.SampleRate())
	fmt.Printf("channels=%d\n", dec.Channels())
	fmt.Printf("bytes=%d\n", len(raw))
}
