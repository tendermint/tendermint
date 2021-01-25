// nolint: gosec
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	baseDir := flag.String("base", ".", `where the "corpus" directory will live`)
	flag.Parse()

	initCorpus(*baseDir)
}

func initCorpus(baseDir string) {
	log.SetFlags(0)

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
		filename := filepath.Join(corpusDir, fmt.Sprintf("%d", i))

		if err := ioutil.WriteFile(filename, []byte(datum), 0644); err != nil {
			log.Fatalf("can't write %v to %q: %v", datum, filename, err)
		}

		log.Printf("wrote %q", filename)
	}
}
