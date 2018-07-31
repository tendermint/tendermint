package autofile

import (
	"io/ioutil"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func TestSIGHUP(t *testing.T) {

	// First, create an AutoFile writing to a tempfile dir
	file, err := ioutil.TempFile("", "sighup_test")
	if err != nil {
		t.Fatalf("Error creating tempfile: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Error closing tempfile: %v", err)
	}
	name := file.Name()
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
	if err := af.Close(); err != nil {
		t.Fatalf("Error closing autofile")
	}

	// Both files should exist
	if body := cmn.MustReadFile(name + "_old"); string(body) != "Line 1\nLine 2\n" {
		t.Errorf("Unexpected body %s", body)
	}
	if body := cmn.MustReadFile(name); string(body) != "Line 3\nLine 4\n" {
		t.Errorf("Unexpected body %s", body)
	}
}
