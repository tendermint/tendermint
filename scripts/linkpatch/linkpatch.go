// Program linkpatch rewrites absolute URLs pointing to targets in GitHub in
// Markdown link tags to target a different branch.
//
// This is used to update documentation links for backport branches.
// See https://github.com/tendermint/tendermint/issues/7675 for context.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/creachadair/atomicfile"
)

var (
	repoName     = flag.String("repo", "tendermint/tendermint", "Repository name to match")
	sourceBranch = flag.String("source", "master", "Source branch name (required)")
	targetBranch = flag.String("target", "", "Target branch name (required)")
	doRecur      = flag.Bool("recur", false, "Recur into subdirectories")

	skipPath  stringList
	skipMatch regexpFlag

	// Match markdown links pointing to absolute URLs.
	// This only works for "inline" links, not referenced links.
	// The submetch selects the URL.
	linkRE = regexp.MustCompile(`(?m)\[.*?\]\((https?://.*?)\)`)
)

func init() {
	flag.Var(&skipPath, "skip-path", "Skip these paths (comma-separated)")
	flag.Var(&skipMatch, "skip-match", "Skip URLs matching this regexp (RE2)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <file/dir>...

Rewrite absolute Markdown links targeting the specified GitHub repository
and source branch name to point to the target branch instead. Matching
files are updated in-place.

Each path names either a directory to list, or a single file path to
rewrite. By default, only the top level of a directory is scanned; use -recur
to recur into subdirectories.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	switch {
	case *repoName == "":
		log.Fatal("You must specify a non-empty -repo name (org/repo)")
	case *targetBranch == "":
		log.Fatal("You must specify a non-empty -target branch")
	case *sourceBranch == "":
		log.Fatal("You must specify a non-empty -source branch")
	case *sourceBranch == *targetBranch:
		log.Fatalf("Source and target branch are the same (%q)", *sourceBranch)
	case flag.NArg() == 0:
		log.Fatal("You must specify at least one file/directory to rewrite")
	}

	r, err := regexp.Compile(fmt.Sprintf(`^https?://github.com/%s/(?:blob|tree)/%s`,
		*repoName, *sourceBranch))
	if err != nil {
		log.Fatalf("Compiling regexp: %v", err)
	}
	for _, path := range flag.Args() {
		if err := processPath(r, path); err != nil {
			log.Fatalf("Processing %q failed: %v", path, err)
		}
	}
}

func processPath(r *regexp.Regexp, path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode().IsDir() {
		return processDir(r, path)
	} else if fi.Mode().IsRegular() {
		return processFile(r, path)
	}
	return nil // nothing to do with links, device files, sockets, etc.
}

func processDir(r *regexp.Regexp, root string) error {
	return filepath.Walk(root, func(path string, fi fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			if skipPath.Contains(path) {
				log.Printf("Skipping %q (per -skip-path)", path)
				return filepath.SkipDir // explicitly skipped
			} else if !*doRecur && path != root {
				return filepath.SkipDir // skipped because we aren't recurring
			}
			return nil // nothing else to do for directories
		} else if skipPath.Contains(path) {
			log.Printf("Skipping %q (per -skip-path)", path)
			return nil // explicitly skipped
		} else if filepath.Ext(path) != ".md" {
			return nil // nothing to do for non-Markdown files
		}

		return processFile(r, path)
	})
}

func processFile(r *regexp.Regexp, path string) error {
	log.Printf("Processing file %q", path)
	input, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	pos := 0
	var output bytes.Buffer
	for _, m := range linkRE.FindAllSubmatchIndex(input, -1) {
		href := string(input[m[2]:m[3]])
		u := r.FindStringIndex(href)
		if u == nil || skipMatch.MatchString(href) {
			if u != nil {
				log.Printf("Skipped URL %q (by -skip-match)", href)
			}
			output.Write(input[pos:m[1]]) // copy the existing data as-is
			pos = m[1]
			continue
		}

		// Copy everything before the URL as-is, then write the replacement.
		output.Write(input[pos:m[2]]) // everything up to the URL
		fmt.Fprintf(&output, `https://github.com/%s/blob/%s%s`, *repoName, *targetBranch, href[u[1]:])

		// Write out the tail of the match, everything after the URL.
		output.Write(input[m[3]:m[1]])
		pos = m[1]
	}
	output.Write(input[pos:]) // the rest of the file

	_, err = atomicfile.WriteAll(path, &output, 0644)
	return err
}

// stringList implements the flag.Value interface for a comma-separated list of strings.
type stringList []string

func (lst *stringList) Set(s string) error {
	if s == "" {
		*lst = nil
	} else {
		*lst = strings.Split(s, ",")
	}
	return nil
}

// Contains reports whether lst contains s.
func (lst stringList) Contains(s string) bool {
	for _, elt := range lst {
		if s == elt {
			return true
		}
	}
	return false
}

func (lst stringList) String() string { return strings.Join([]string(lst), ",") }

// regexpFlag implements the flag.Value interface for a regular expression.
type regexpFlag struct{ *regexp.Regexp }

func (r regexpFlag) MatchString(s string) bool {
	if r.Regexp == nil {
		return false
	}
	return r.Regexp.MatchString(s)
}

func (r *regexpFlag) Set(s string) error {
	c, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	r.Regexp = c
	return nil
}

func (r regexpFlag) String() string {
	if r.Regexp == nil {
		return ""
	}
	return r.Regexp.String()
}
