// Program findbuiltin locates calls to built-in functions in Go source
// code, and writes a machine-readable log of where those calls occur.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tendermint/tendermint/tools/panic/bicall"
)

var (
	matchNames    []string // function names to match
	doPipe        bool     // read paths from stdin
	doSkipMissing bool     // do not fail for missing files
)

func init() {
	flag.Var(stringList{&matchNames}, "match", `Comma-separated function names to select ("" for all)`)
	flag.BoolVar(&doPipe, "pipe", false, "Read paths from stdin, one per line")
	flag.BoolVar(&doSkipMissing, "skip-missing", false, "Ignore input paths that are not found")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] path...  : process the named source files
       %[1]s [options] -pipe    : process paths from stdin

Scan the specified Go source files for calls to built-in functions, and print a
log of those calls to stdout.

With -pipe, the program reads paths from stdin, one path per line.
In this case, paths given on the command-line are consumed first.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// The default input consists of the command-line arguments.
	// If -pipe is given, also concatenate the contents of stdin.
	var in io.Reader = strings.NewReader(strings.Join(flag.Args(), "\n"))
	if doPipe {
		in = io.MultiReader(in, os.Stdin)
	}

	lines := bufio.NewScanner(in)
	for lines.Scan() {
		mustProcessFile(lines.Text())
	}
	if err := lines.Err(); err != nil {
		log.Fatalf("Error scanning: %v", err)
	}
}

func mustProcessFile(path string) {
	f, err := os.Open(path)
	if os.IsNotExist(err) && doSkipMissing {
		log.Printf("File not found: %q [skipped]", path)
		return
	} else if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := bicall.Parse(f, path, func(c bicall.Call) error {
		if !wantFunction(c.Name) {
			return nil
		}
		bits, err := json.Marshal(struct {
			Name string   `json:"name"`
			Path string   `json:"path"`
			Line int      `json:"line"`
			Col  int      `json:"col"`
			Com  []string `json:"comments"`
		}{
			Name: c.Name,
			Path: c.Site.Filename,
			Line: c.Site.Line,
			Col:  c.Site.Column - 1,
			Com:  c.Comments,
		})
		if err != nil {
			log.Fatalf("Marshaling output: %v", err)
		}
		fmt.Println(string(bits))
		return nil
	}); err != nil {
		log.Fatalf("Parsing %q failed: %v", path, err)
	}
}

func wantFunction(name string) bool {
	if len(matchNames) == 0 {
		return true
	}
	for _, want := range matchNames {
		if want == name {
			return true
		}
	}
	return false
}

type stringList struct{ v *[]string }

func (s stringList) Set(v string) error {
	ss := strings.Split(v, ",")
	if len(ss) == 1 && ss[0] == "" {
		*s.v = nil
	} else {
		*s.v = ss
	}
	return nil
}

func (s stringList) String() string {
	if s.v == nil {
		return ""
	}
	return strings.Join(*s.v, ",")
}
