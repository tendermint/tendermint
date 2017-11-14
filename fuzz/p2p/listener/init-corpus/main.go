package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	rootDirVar := flag.String("root", ".", `the directory in which the "corpus", "testdata" directories are`)
	flag.Parse()
	rootDir := *rootDirVar
	if rootDir == "" {
		rootDir = "."
	}
	InitCorpus(rootDir)
}

func InitCorpus(rootDir string) {
	log.SetFlags(0)
	corpusDir := filepath.Join(rootDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		log.Fatalf("corpus directory: %v", err)
	}

	addresses := []string{
		// Only seeing with TCP addresses
		"tcp,:8988",
		"tcp,0.0.0.0:8988",
		"tcp,127.0.0.0:8798",
		"tcp,0.0.0.0:46657",
		"tcp,localhost:46657",
	}

	for i, address := range addresses {
		fullPath := filepath.Join(corpusDir, fmt.Sprintf("%d", i))
		f, err := os.Create(fullPath)
		if err == nil {
			fmt.Fprintf(f, address)
			f.Close()
			log.Printf("#%d: Successfully wrote %q", i, fullPath)
		} else {
			log.Printf("#%d: %q err: %v", i, fullPath, err)
		}
	}
}
