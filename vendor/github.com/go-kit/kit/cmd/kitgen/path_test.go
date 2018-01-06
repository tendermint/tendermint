package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestImportPath(t *testing.T) {
	testcase := func(gopath, targetpath, expected string) {
		t.Run(fmt.Sprintf("%q + %q", gopath, targetpath), func(t *testing.T) {
			actual, err := importPath(targetpath, gopath)
			if err != nil {
				t.Fatalf("Expected no error, got %q", err)
			}
			if actual != expected {
				t.Errorf("Expected %q, got %q", expected, actual)
			}
		})
	}

	testcase("/gopath/", "/gopath/src/somewhere", "somewhere")
	testcase("/gopath", "/gopath/src/somewhere", "somewhere")
	testcase("/gopath:/other", "/gopath/src/somewhere", "somewhere")
	testcase("/other:/gopath/", "/gopath/src/somewhere", "somewhere")
}

func TestImportPathSadpath(t *testing.T) {
	testcase := func(gopath, targetpath, expected string) {
		t.Run(fmt.Sprintf("%q + %q", gopath, targetpath), func(t *testing.T) {
			actual, err := importPath(targetpath, gopath)
			if actual != "" {
				t.Errorf("Expected empty path, got %q", actual)
			}
			if strings.Index(err.Error(), expected) == -1 {
				t.Errorf("Expected %q to include %q", err, expected)
			}
		})
	}

	testcase("", "/gopath/src/somewhere", "is not in")
	testcase("", "./somewhere", "not an absolute")
}
