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
	path = gopath()
	if path != "~/testgopath" {
		t.Fatalf("gopath should return GOPATH env var if set, got %v", path)
	}
	os.Unsetenv("GOPATH")

	path = gopath()
	if path == "~/testgopath" || path == "" {
		t.Fatalf("gopath should return go env GOPATH result if env var does not exist, got %v", path)
	}
}
