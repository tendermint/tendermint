// Program confix applies fixes to a Tendermint TOML configuration file to
// update a file created with an older version of Tendermint to a compatible
// format for a newer version.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/transform"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/config"
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
)

func main() {
	flag.Parse()
	if *configPath == "" {
		log.Fatal("You must specify a non-empty -config path")
	}

	doc, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	ctx := transform.WithLogWriter(context.Background(), os.Stderr)
	if err := ApplyFixes(ctx, doc); err != nil {
		log.Fatalf("Updating %q: %v", *configPath, err)
	}

	var buf bytes.Buffer
	if err := tomledit.Format(&buf, doc); err != nil {
		log.Fatalf("Formatting config: %v", err)
	}

	// Verify that Tendermint can parse the results after our edits.
	if err := CheckValid(buf.Bytes()); err != nil {
		log.Fatalf("Updated config is invalid: %v", err)
	}

	if *outPath == "" {
		os.Stdout.Write(buf.Bytes())
	} else if err := atomicfile.WriteData(*outPath, buf.Bytes(), 0600); err != nil {
		log.Fatalf("Writing output: %v", err)
	}
}

// ApplyFixes transforms doc and reports whether it succeeded.
func ApplyFixes(ctx context.Context, doc *tomledit.Document) error {
	// Check what version of Tendermint might have created this config file, as
	// a safety check for the updates we are about to make.
	tmVersion := GuessConfigVersion(doc)
	if tmVersion == vUnknown {
		return errors.New("cannot tell what Tendermint version created this config")
	} else if tmVersion < v34 || tmVersion > v36 {
		// TODO(creachadair): Add in rewrites for older versions.  This will
		// require some digging to discover what the changes were.  The upgrade
		// instructions do not give specifics.
		return fmt.Errorf("unable to update version %s config", tmVersion)
	}
	return plan.Apply(ctx, doc)
}

// LoadConfig loads and parses the TOML document from path.
func LoadConfig(path string) (*tomledit.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return tomledit.Parse(f)
}

const (
	vUnknown = ""
	v32      = "v0.32"
	v33      = "v0.33"
	v34      = "v0.34"
	v35      = "v0.35"
	v36      = "v0.36"
)

// GuessConfigVersion attempts to figure out which version of Tendermint
// created the specified config document. It returns "" if the creating version
// cannot be determined, otherwise a string of the form "vX.YY".
func GuessConfigVersion(doc *tomledit.Document) string {
	hasDisableWS := doc.First("rpc", "experimental-disable-websocket") != nil
	hasUseLegacy := doc.First("p2p", "use-legacy") != nil // v0.35 only
	if hasDisableWS && !hasUseLegacy {
		return v36
	}

	hasBlockSync := transform.FindTable(doc, "blocksync") != nil // add: v0.35
	hasStateSync := transform.FindTable(doc, "statesync") != nil // add: v0.34
	if hasBlockSync && hasStateSync {
		return v35
	} else if hasStateSync {
		return v34
	}

	hasIndexKeys := doc.First("tx_index", "index_keys") != nil // add: v0.33
	hasIndexTags := doc.First("tx_index", "index_tags") != nil // rem: v0.33
	if hasIndexKeys && !hasIndexTags {
		return v33
	}

	hasFastSync := transform.FindTable(doc, "fastsync") != nil // add: v0.32
	if hasIndexTags && hasFastSync {
		return v32
	}

	// Something older, probably.
	return vUnknown
}

// CheckValid checks whether the specified config appears to be a valid
// Tendermint config file. This emulates how the node loads the config.
func CheckValid(data []byte) error {
	v := viper.New()
	v.SetConfigType("toml")

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("decoding config: %w", err)
	}

	return cfg.ValidateBasic()
}
