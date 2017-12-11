package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	baseDir := flag.String("base", ".", "the base directory")
	flag.Parse()

	if err := initCorpus(*baseDir); err != nil {
		log.Fatal(err)
	}
}

func initCorpus(baseDir string) error {
	corpusDir := filepath.Join(baseDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		return err
	}
	sequences := [][]uint64{
		{0, 10, 1<<64 - 1, 999981},
		{17},
		{37, 0, 0, 0},
		{15, 99, 200, 0},
	}

	for i, seq := range sequences {
		bs := make([]byte, len(seq)*8)
		for j, item := range seq {
			binary.BigEndian.PutUint64(bs[j*8:], item)
		}
		outPath := filepath.Join(corpusDir, fmt.Sprintf("%d", i))
		f, err := os.Create(outPath)
		if err != nil {
			log.Printf("#%d: failed to create %q: %v", i, outPath, err)
		} else {
			f.Write(bs)
			f.Close()
		}
	}
	return nil
}
