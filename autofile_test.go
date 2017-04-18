package autofile

import (
	. "github.com/tendermint/go-common"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestSIGHUP(t *testing.T) {

	// First, create an AutoFile writing to a tempfile dir
	file, name := Tempfile("sighup_test")
	err := file.Close()
	if err != nil {
		t.Fatalf("Error creating tempfile: %v", err)
	}
	// Here is the actual AutoFile
	af, err := OpenAutoFile(name)
	if err != nil {
		t.Fatalf("Error creating autofile: %v", err)
	}

	// Write to the file.
	_, err = af.Write([]byte("Line 1\n"))
	if err != nil {
		t.Fatalf("Error writing to autofile: %v", err)
	}
	_, err = af.Write([]byte("Line 2\n"))
	if err != nil {
		t.Fatalf("Error writing to autofile: %v", err)
	}

	// Move the file over
	err = os.Rename(name, name+"_old")
	if err != nil {
		t.Fatalf("Error moving autofile: %v", err)
	}

	// Send SIGHUP to self.
	oldSighupCounter := atomic.LoadInt32(&sighupCounter)
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)

	// Wait a bit... signals are not handled synchronously.
	for atomic.LoadInt32(&sighupCounter) == oldSighupCounter {
		time.Sleep(time.Millisecond * 10)
	}

	// Write more to the file.
	_, err = af.Write([]byte("Line 3\n"))
	if err != nil {
		t.Fatalf("Error writing to autofile: %v", err)
	}
	_, err = af.Write([]byte("Line 4\n"))
	if err != nil {
		t.Fatalf("Error writing to autofile: %v", err)
	}
	err = af.Close()
	if err != nil {
		t.Fatalf("Error closing autofile")
	}

	// Both files should exist
	if body := MustReadFile(name + "_old"); string(body) != "Line 1\nLine 2\n" {
		t.Errorf("Unexpected body %s", body)
	}
	if body := MustReadFile(name); string(body) != "Line 3\nLine 4\n" {
		t.Errorf("Unexpected body %s", body)
	}

}
