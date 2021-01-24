package os_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	cmd, _, mockStderr := newTestProgram(t, "TM_TRAP_SIGNAL_TEST")

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

func TestEnsureDir(t *testing.T) {
	tmp, err := ioutil.TempDir("", "ensure-dir")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	// Should be possible to create a new directory.
	err = tmos.EnsureDir(filepath.Join(tmp, "dir"), 0755)
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(tmp, "dir"))

	// Should succeed on existing directory.
	err = tmos.EnsureDir(filepath.Join(tmp, "dir"), 0755)
	require.NoError(t, err)

	// Should fail on file.
	err = ioutil.WriteFile(filepath.Join(tmp, "file"), []byte{}, 0644)
	require.NoError(t, err)
	err = tmos.EnsureDir(filepath.Join(tmp, "file"), 0755)
	require.Error(t, err)

	// Should allow symlink to dir.
	err = os.Symlink(filepath.Join(tmp, "dir"), filepath.Join(tmp, "linkdir"))
	require.NoError(t, err)
	err = tmos.EnsureDir(filepath.Join(tmp, "linkdir"), 0755)
	require.NoError(t, err)

	// Should error on symlink to file.
	err = os.Symlink(filepath.Join(tmp, "file"), filepath.Join(tmp, "linkfile"))
	require.NoError(t, err)
	err = tmos.EnsureDir(filepath.Join(tmp, "linkfile"), 0755)
	require.Error(t, err)
}

type mockLogger struct{}

func (ml mockLogger) Info(msg string, keyvals ...interface{}) {}

func killer() {
	logger := mockLogger{}

	tmos.TrapSignal(logger, func() { _, _ = fmt.Fprintf(os.Stderr, "exiting") })
	time.Sleep(1 * time.Second)

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic(err)
	}

	if err := p.Signal(syscall.SIGTERM); err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
}

func newTestProgram(t *testing.T, environVar string) (cmd *exec.Cmd, stdout *bytes.Buffer, stderr *bytes.Buffer) {
	t.Helper()

	cmd = exec.Command(os.Args[0], "-test.run="+t.Name())
	stdout, stderr = bytes.NewBufferString(""), bytes.NewBufferString("")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", environVar))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return
}
