package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	. "github.com/tendermint/tendermint/common"
)

const Version = "0.0.1"
const readBufferSize = 1024

// Parse command-line options
func parseFlags() (outpath string, version bool) {
	flag.StringVar(&outpath, "outpath", "stdinwriter.out", "Output file name")
	flag.BoolVar(&version, "version", false, "Version")
	flag.Parse()
	return
}

func main() {

	// Read options
	outpath, version := parseFlags()
	if version {
		fmt.Println(Fmt("stdinwriter version %v", Version))
		return
	}

	outfile, err := OpenAutoFile(outpath)
	if err != nil {
		fmt.Println(Fmt("stdinwriter couldn't create outfile %v", outfile))
		os.Exit(1)
	}

	go writeToOutfile(outfile)

	// Trap signal
	TrapSignal(func() {
		outfile.Close()
		fmt.Println("stdinwriter shutting down")
	})
}

func writeToOutfile(outfile *AutoFile) {
	// Forever, read from stdin and write to AutoFile.
	buf := make([]byte, readBufferSize)
	for {
		n, err := os.Stdin.Read(buf)
		outfile.Write(buf[:n])
		if err != nil {
			outfile.Close()
			if err == io.EOF {
				os.Exit(0)
			} else {
				fmt.Println("stdinwriter errored")
				os.Exit(1)
			}
		}
	}
}
