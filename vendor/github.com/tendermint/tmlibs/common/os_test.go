package common

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestWriteFileAtomic(t *testing.T) {
	data := []byte("Becatron")
	fname := fmt.Sprintf("/tmp/write-file-atomic-test-%v.txt", time.Now().UnixNano())
	err := WriteFileAtomic(fname, data, 0664)
	if err != nil {
		t.Fatal(err)
	}
	rData, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, rData) {
		t.Fatalf("data mismatch: %v != %v", data, rData)
	}
	if err := os.Remove(fname); err != nil {
		t.Fatal(err)
	}
}

func TestGoPath(t *testing.T) {
	// restore original gopath upon exit
	path := os.Getenv("GOPATH")
	defer func() {
		_ = os.Setenv("GOPATH", path)
	}()

	err := os.Setenv("GOPATH", "~/testgopath")
	if err != nil {
		t.Fatal(err)
	}
	path = GoPath()
	if path != "~/testgopath" {
		t.Fatalf("should get GOPATH env var value, got %v", path)
	}
	os.Unsetenv("GOPATH")

	path = GoPath()
	if path != "~/testgopath" {
		t.Fatalf("subsequent calls should return the same value, got %v", path)
	}
}

func TestGoPathWithoutEnvVar(t *testing.T) {
	// restore original gopath upon exit
	path := os.Getenv("GOPATH")
	defer func() {
		_ = os.Setenv("GOPATH", path)
	}()

	os.Unsetenv("GOPATH")
	// reset cache
	gopath = ""

	path = GoPath()
	if path == "" || path == "~/testgopath" {
		t.Fatalf("should get nonempty result of calling go env GOPATH, got %v", path)
	}
}
