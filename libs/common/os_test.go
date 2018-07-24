package common

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	var (
		data             = []byte(RandStr(RandIntn(2048)))
		old              = RandBytes(RandIntn(2048))
		perm os.FileMode = 0600
	)

	f, err := ioutil.TempFile("/tmp", "write-atomic-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if err = ioutil.WriteFile(f.Name(), old, 0664); err != nil {
		t.Fatal(err)
	}

	if err = WriteFileAtomic(f.Name(), data, perm); err != nil {
		t.Fatal(err)
	}

	rData, err := ioutil.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, rData) {
		t.Fatalf("data mismatch: %v != %v", data, rData)
	}

	stat, err := os.Stat(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if have, want := stat.Mode().Perm(), perm; have != want {
		t.Errorf("have %v, want %v", have, want)
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
