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
	"strings"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
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

var plan = transform.Plan{
	{
		// Since https://github.com/tendermint/tendermint/pull/5777.
		Desc: "Rename everything from snake_case to kebab-case",
		T:    transform.SnakeToKebab(),
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6896.
		Desc:    "Rename [fastsync] to [blocksync]",
		T:       transform.Rename(parser.Key{"fastsync"}, parser.Key{"blocksync"}),
		ErrorOK: true,
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/7159.
		Desc: "Move top-level fast_sync key to blocksync.enable",
		T: transform.MoveKey(
			parser.Key{"fast-sync"},
			parser.Key{"blocksync"},
			parser.Key{"enable"},
		),
		ErrorOK: true,
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6241.
		Desc: `Add top-level mode setting (default "full")`,
		T: transform.EnsureKey(nil, &parser.KeyValue{
			Block: parser.Comments{"Mode of Node: full | validator | seed"},
			Name:  parser.Key{"mode"},
			Value: parser.MustValue(`"full"`),
		}),
		ErrorOK: true,
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6396.
		Desc:    "Remove vestigial mempool.wal-dir setting",
		T:       transform.Remove(parser.Key{"mempool", "wal-dir"}),
		ErrorOK: true,
	},
	{
		// Added in https://github.com/tendermint/tendermint/pull/6466.
		Desc: `Add mempool.version default to "v1"`,
		T: transform.EnsureKey(parser.Key{"mempool"}, &parser.KeyValue{
			Block: parser.Comments{`Mempool version to use`},
			Name:  parser.Key{"version"},
			Value: parser.MustValue(`"v1"`),
		}),
		ErrorOK: true,
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6323.
		Desc: "Add new [p2p] queue-type setting",
		T: transform.EnsureKey(parser.Key{"p2p"}, &parser.KeyValue{
			Block: parser.Comments{"Select the p2p internal queue"},
			Name:  parser.Key{"queue-type"},
			Value: parser.MustValue(`"priority"`),
		}),
		ErrorOK: true,
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6353.
		Desc: "Add [p2p] connection count and rate limit settings",
		T: transform.Func(func(_ context.Context, doc *tomledit.Document) error {
			tab := transform.FindTable(doc, "p2p")
			if tab == nil {
				return errors.New("p2p table not found")
			}
			transform.InsertMapping(tab.Section, &parser.KeyValue{
				Block: parser.Comments{"Maximum number of connections (inbound and outbound)."},
				Name:  parser.Key{"max-connections"},
				Value: parser.MustValue("64"),
			}, false)
			transform.InsertMapping(tab.Section, &parser.KeyValue{
				Block: parser.Comments{
					"Rate limits the number of incoming connection attempts per IP address.",
				},
				Name:  parser.Key{"max-incoming-connection-attempts"},
				Value: parser.MustValue("100"),
			}, false)
			return nil
		}),
	},
	{
		// Added "chunk-fetchers" https://github.com/tendermint/tendermint/pull/6566.
		// This value was backported into v0.34.11 (modulo casing).
		// Renamed to "fetchers"  https://github.com/tendermint/tendermint/pull/6587.
		Desc: "Rename statesync.chunk-fetchers to statesync.fetchers",
		T: transform.Func(func(ctx context.Context, doc *tomledit.Document) error {
			// If the key already exists, rename it preserving its value.
			if found := doc.First("statesync", "chunk-fetchers"); found != nil {
				found.KeyValue.Name = parser.Key{"fetchers"}
				return nil
			}

			// Otherwise, add it.
			return transform.EnsureKey(parser.Key{"statesync"}, &parser.KeyValue{
				Block: parser.Comments{
					"The number of concurrent chunk and block fetchers to run (default: 4).",
				},
				Name:  parser.Key{"fetchers"},
				Value: parser.MustValue("4"),
			})(ctx, doc)
		}),
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6807.
		// Backported into v0.34.13 (modulo casing).
		Desc: "Add statesync.use-p2p setting",
		T: transform.EnsureKey(parser.Key{"statesync"}, &parser.KeyValue{
			Block: parser.Comments{
				"# State sync uses light client verification to verify state. This can be done either through the",
				"# P2P layer or RPC layer. Set this to true to use the P2P layer. If false (default), RPC layer",
				"# will be used.",
			},
			Name:  parser.Key{"use-p2p"},
			Value: parser.MustValue("false"),
		}),
	},
	{
		// Since https://github.com/tendermint/tendermint/pull/6462.
		Desc: "Move priv-validator settings under [priv-validator]",
		T: transform.Func(func(_ context.Context, doc *tomledit.Document) error {
			const pvPrefix = "priv-validator-"

			var found []*tomledit.Entry
			doc.Scan(func(key parser.Key, e *tomledit.Entry) bool {
				if e.IsSection() && !e.IsGlobal() {
					return false // no more candidates
				} else if len(key) == 1 && strings.HasPrefix(key[0], pvPrefix) {
					found = append(found, e)
				}
				return true
			})
			if len(found) == 0 {
				return nil // nothing to do
			}

			// Now that we know we have work to do, find the target table.
			var sec *tomledit.Section
			if dst := transform.FindTable(doc, "priv-validator"); dst == nil {
				// If the table doesn't exist, create it. Old config files
				// probably will not have it, so plug in the comment too.
				sec = &tomledit.Section{
					Heading: &parser.Heading{
						Block: parser.Comments{
							"#######################################################",
							"###       Priv Validator Configuration              ###",
							"#######################################################",
						},
						Name: parser.Key{"priv-validator"},
					},
				}
				doc.Sections = append(doc.Sections, sec)
			} else {
				sec = dst.Section
			}

			for _, e := range found {
				e.Remove()
				e.Name = parser.Key{strings.TrimPrefix(e.Name[0], pvPrefix)}
				sec.Items = append(sec.Items, e.KeyValue)
			}
			return nil
		}),
	},
	{
		// v1 removed: https://github.com/tendermint/tendermint/pull/5728
		// v2 deprecated: https://github.com/tendermint/tendermint/pull/6730
		Desc: `Set blocksync.version to "v0"`,
		T: transform.Func(func(_ context.Context, doc *tomledit.Document) error {
			v := doc.First("blocksync", "version")
			if v == nil {
				return nil // nothing to do
			} else if !v.IsMapping() {
				// This shouldn't happen, but is easier to debug than a panic.
				return fmt.Errorf("blocksync.version is weird: %v", v)
			}
			v.Value.X = parser.MustValue(`"v0"`).X
			return nil
		}),
	},
}

// ApplyFixes transforms doc and reports whether it succeeded.
func ApplyFixes(ctx context.Context, doc *tomledit.Document) error {
	// Check what version of Tendermint might have created this config file, as
	// a safety check for the updates we are about to make.
	tmVersion := GuessConfigVersion(doc)
	if tmVersion == vUnknown {
		return errors.New("cannot tell what Tendermint version created this config")
	} else if tmVersion < v34 || tmVersion > v35 {
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

	// Stub in required value not stored in the config file, so that validation
	// will not fail spuriously.
	cfg.Mode = config.ModeValidator
	return cfg.ValidateBasic()
}
