package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	baseDir := flag.String("base", ".", "the base directory in which the corpus directory resides")
	flag.Parse()

	log.SetFlags(0)
	InitCorpus(*baseDir)
}

func InitCorpus(baseDir string) {
	corpusDir := filepath.Join(baseDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		log.Fatal(err)
	}

	data := []string{
		"dadc04c2-cfb1-4aa9-a92a-c0bf780ec8b6",
		"",
		" ",
		"           a                                   ",
		`{"a": 12, "tsp": 999, k: "blue"}`,
		`9999.999`,
		`""`,
		`Tendermint fuzzing`,
	}

	for i, datum := range data {
		fullPath := filepath.Join(corpusDir, fmt.Sprintf("%d", i))
		f, err := os.Create(fullPath)
		if err == nil {
			f.Write([]byte(datum))
			f.Close()
			log.Printf("#%d Successfully wrote %q", i, fullPath)
		} else {
			log.Printf("#%d %q err:%v", i, fullPath, err)
		}
	}
}
