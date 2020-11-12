package os_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	tmos "github.com/tendermint/tendermint/libs/os"
)

func TestCopyFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	content := []byte("hello world")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}

	copyfile := fmt.Sprintf("%s.copy", tmpfile.Name())
	if err := tmos.CopyFile(tmpfile.Name(), copyfile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyfile); os.IsNotExist(err) {
		t.Fatal("copy should exist")
	}
	data, err := ioutil.ReadFile(copyfile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("copy file content differs: expected %v, got %v", content, data)
	}
	os.Remove(copyfile)
}

func TestTrapSignal(t *testing.T) {
	if os.Getenv("TM_TRAP_SIGNAL_TEST") == "1" {
		t.Log("inside test process")
		killer()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	mockStderr := bytes.NewBufferString("")
	cmd.Env = append(os.Environ(), "TM_TRAP_SIGNAL_TEST=1")
	cmd.Stderr = mockStderr

	err := cmd.Run()
	if err == nil {
		wantStderr := "exiting"
		if mockStderr.String() != wantStderr {
			t.Fatalf("stderr: want %q, got %q", wantStderr, mockStderr.String())
		}

		return
	}

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("wrong exit code, want 0, got %d", e.ExitCode())
	}

	t.Fatal("this error should not be triggered")

}

type mockLogger struct{}

func (ml mockLogger) Info(msg string, keyvals ...interface{}) {}

func killer() {
	logger := mockLogger{}

	tmos.TrapSignal(logger, func() { _, _ = fmt.Fprintf(os.Stderr, "exiting") })
	time.Sleep(1 * time.Second)

	// use Kill() to test SIGTERM
	if err := tmos.Kill(); err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
}
