// Program confix applies fixes to a Tendermint TOML configuration file to
// update a file created with an older version of Tendermint to a compatible
// format for a newer version.
package main

import (
	"context"
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
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] -config <path>

Modify the contents of the specified -config TOML file to update the
names, locations, and values of configuration settings to the current
configuration layout.

By default, the config file is edited in-place; use -out to write the
modified file to a different path. In case of any error in updating the
file, the input is not modified.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

var (
	configPath = flag.String("config", "", "Config file path (required)")
	outPath    = flag.String("out", "", "Output file path (defaults to -input)")

	plan = transform.Plan{
		{
			Desc: "Rename everything from snake_case to kebab-case",
			T:    transform.SnakeToKebab(),
		},
		{
			Desc:    "Rename [fastsync] to [blocksync]",
			T:       transform.Rename(parser.Key{"fastsync"}, parser.Key{"blocksync"}),
			ErrorOK: true,
		},
		{
			Desc: "Move top-level fast_sync key under [blocksync]",
			T: transform.MoveKey(
				parser.Key{"fast-sync"},
				parser.Key{"blocksync"},
				parser.Key{"fast-sync"},
			),
			ErrorOK: true,
		},
		{
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
)

func main() {
	flag.Parse()
	if *configPath == "" {
		log.Fatal("You must specify a non-empty -config path")
	} else if *outPath == "" {
		*outPath = *configPath
	}

	doc, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	ctx := transform.WithLogWriter(context.Background(), os.Stderr)
	if err := plan.Apply(ctx, doc); err != nil {
		log.Fatalf("Editing config: %v", err)
	}

	out, err := atomicfile.New(*outPath, 0600)
	if err != nil {
		log.Fatalf("Open output: %v", err)
	}
	defer out.Cancel()

	if err := tomledit.Format(out, doc); err != nil {
		log.Fatalf("Writing config: %v", err)
	} else if err := out.Close(); err != nil {
		log.Fatalf("Closing output: %v", err)
	}
}

func loadConfig(path string) (*tomledit.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return tomledit.Parse(f)
}
