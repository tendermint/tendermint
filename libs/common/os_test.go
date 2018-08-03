package common

import (
	"os"
	"testing"
)

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
