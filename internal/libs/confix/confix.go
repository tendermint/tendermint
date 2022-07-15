// Package confix applies changes to a Tendermint TOML configuration file, to
// update configurations created with an older version of Tendermint to a
// compatible format for a newer version.
package confix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/transform"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/config"
)

// Upgrade reads the configuration file at configPath and applies any
// transformations necessary to upgrade it to the current version.  If this
// succeeds, the transformed output is written to outputPath. As a special
// case, if outputPath == "" the output is written to stdout.
//
// It is safe if outputPath == inputPath. If a regular file outputPath already
// exists, it is overwritten. In case of error, the output is not written.
//
// Upgrade is a convenience wrapper for calls to LoadConfig, ApplyFixes, and
// CheckValid. If the caller requires more control over the behavior of the
// upgrade, call those functions directly.
func Upgrade(ctx context.Context, configPath, outputPath string) error {
	if configPath == "" {
		return errors.New("empty input configuration path")
	}

	doc, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %v", err)
	}

	if err := ApplyFixes(ctx, doc); err != nil {
		return fmt.Errorf("updating %q: %v", configPath, err)
	}

	var buf bytes.Buffer
	if err := tomledit.Format(&buf, doc); err != nil {
		return fmt.Errorf("formatting config: %v", err)
	}

	// Verify that Tendermint can parse the results after our edits.
	if err := CheckValid(buf.Bytes()); err != nil {
		return fmt.Errorf("updated config is invalid: %v", err)
	}

	if outputPath == "" {
		_, err = os.Stdout.Write(buf.Bytes())
	} else {
		err = atomicfile.WriteData(outputPath, buf.Bytes(), 0600)
	}
	return err
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

// WithLogWriter returns a child of ctx with a logger attached that sends
// output to w. This is a convenience wrapper for transform.WithLogWriter.
func WithLogWriter(ctx context.Context, w io.Writer) context.Context {
	return transform.WithLogWriter(ctx, w)
}
