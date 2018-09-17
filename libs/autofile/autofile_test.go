package autofile

import (
	"io/ioutil"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/errors"
)

func TestSIGHUP(t *testing.T) {
	// First, create an AutoFile writing to a tempfile dir
	file, err := ioutil.TempFile("", "sighup_test")
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	name := file.Name()

	// Here is the actual AutoFile
	af, err := OpenAutoFile(name)
	require.NoError(t, err)

	// Write to the file.
	_, err = af.Write([]byte("Line 1\n"))
	require.NoError(t, err)
	_, err = af.Write([]byte("Line 2\n"))
	require.NoError(t, err)

	// Move the file over
	err = os.Rename(name, name+"_old")
	require.NoError(t, err)

	// Send SIGHUP to self.
	oldSighupCounter := atomic.LoadInt32(&sighupCounter)
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)

	// Wait a bit... signals are not handled synchronously.
	for atomic.LoadInt32(&sighupCounter) == oldSighupCounter {
		time.Sleep(time.Millisecond * 10)
	}

	// Write more to the file.
	_, err = af.Write([]byte("Line 3\n"))
	require.NoError(t, err)
	_, err = af.Write([]byte("Line 4\n"))
	require.NoError(t, err)
	err = af.Close()
	require.NoError(t, err)

	// Both files should exist
	if body := cmn.MustReadFile(name + "_old"); string(body) != "Line 1\nLine 2\n" {
		t.Errorf("Unexpected body %s", body)
	}
	if body := cmn.MustReadFile(name); string(body) != "Line 3\nLine 4\n" {
		t.Errorf("Unexpected body %s", body)
	}
}

// Manually modify file permissions, close, and reopen using autofile:
// We expect the file permissions to be changed back to the intended perms.
func TestOpenAutoFilePerms(t *testing.T) {
	file, err := ioutil.TempFile("", "permission_test")
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	name := file.Name()

	// open and change permissions
	af, err := OpenAutoFile(name)
	require.NoError(t, err)
	err = af.file.Chmod(0755)
	require.NoError(t, err)
	err = af.Close()
	require.NoError(t, err)

	// reopen and expect an ErrPermissionsChanged as Cause
	af, err = OpenAutoFile(name)
	require.Error(t, err)
	if e, ok := err.(*errors.ErrPermissionsChanged); ok {
		t.Logf("%v", e)
	} else {
		t.Errorf("unexpected error %v", e)
	}

	err = af.Close()
	require.NoError(t, err)
}
