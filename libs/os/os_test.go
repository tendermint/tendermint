package os_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
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
	if os.Getenv("TRAP_SIGNAL_TEST") == "1" {
		t.Log("inside test process")
		killer()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	mockStderr := bytes.NewBufferString("")
	cmd.Env = append(os.Environ(), "TRAP_SIGNAL_TEST=1")
	cmd.Stderr = mockStderr

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		want := int(syscall.SIGTERM) + 128
		if e.ExitCode() != int(syscall.SIGTERM)+128 {
			t.Fatalf("wrong exit code, want %d, got %d", want, e.ExitCode())
		}

		return
	}

	t.Fatal("this error should not be triggered")

}

type mockLogger struct{}

func (ml mockLogger) Info(msg string, keyvals ...interface{}) {}

func killer() {
	logger := mockLogger{}

	tmos.TrapSignal(logger, nil)
	time.Sleep(1 * time.Second)

	// use Kill() to test SIGTERM
	if err := tmos.Kill(); err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
}
