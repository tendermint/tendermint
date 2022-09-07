// Program confix applies changes to a Tendermint TOML configuration file, to
// update configurations created with an older version of Tendermint to a
// compatible format for a newer version.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/internal/libs/confix"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s -config <src> [-out <dst>]

Modify the contents of the specified -config TOML file to update the names,
locations, and values of configuration settings to the current configuration
layout. The output is written to -out, or to stdout.

It is valid to set -config and -out to the same path. In that case, the file will
be modified in-place. In case of any error in updating the file, no output is
written.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

var (
	configPath = flag.String("config", "", "Config file path (required)")
	outPath    = flag.String("out", "", "Output file path (default stdout)")
	doVerbose  = flag.Bool("v", false, "Log changes to stderr")
)

func main() {
	flag.Parse()
	if *configPath == "" {
		log.Fatal("You must specify a non-empty -config path")
	}

	ctx := context.Background()
	if *doVerbose {
		ctx = confix.WithLogWriter(ctx, os.Stderr)
	}
	if err := confix.Upgrade(ctx, *configPath, *outPath); err != nil {
		log.Fatalf("Upgrading config: %v", err)
	}
}
