package autofile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSIGHUP(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Error(err)
		}
	})

	// First, create a temporary directory and move into it
	dir, err := os.MkdirTemp("", "sighup_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	require.NoError(t, os.Chdir(dir))

	// Create an AutoFile in the temporary directory
	name := "sighup_test"
	af, err := OpenAutoFile(name)
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(af.Path))

	// Write to the file.
	_, err = af.Write([]byte("Line 1\n"))
	require.NoError(t, err)
	_, err = af.Write([]byte("Line 2\n"))
	require.NoError(t, err)

	// Move the file over
	require.NoError(t, os.Rename(name, name+"_old"))

	// Move into a different temporary directory
	otherDir, err := os.MkdirTemp("", "sighup_test_other")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(otherDir) })
	require.NoError(t, os.Chdir(otherDir))

	// Send SIGHUP to self.
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGHUP))

	// Wait a bit... signals are not handled synchronously.
	time.Sleep(time.Millisecond * 10)

	// Write more to the file.
	_, err = af.Write([]byte("Line 3\n"))
	require.NoError(t, err)
	_, err = af.Write([]byte("Line 4\n"))
	require.NoError(t, err)
	require.NoError(t, af.Close())

	// Both files should exist
	if body := mustReadFile(t, filepath.Join(dir, name+"_old")); string(body) != "Line 1\nLine 2\n" {
		t.Errorf("unexpected body %s", body)
	}
	if body := mustReadFile(t, filepath.Join(dir, name)); string(body) != "Line 3\nLine 4\n" {
		t.Errorf("unexpected body %s", body)
	}

	// The current directory should be empty
	files, err := os.ReadDir(".")
	require.NoError(t, err)
	assert.Empty(t, files)
}

// // Manually modify file permissions, close, and reopen using autofile:
// // We expect the file permissions to be changed back to the intended perms.
// func TestOpenAutoFilePerms(t *testing.T) {
// 	file, err := os.CreateTemp("", "permission_test")
// 	require.NoError(t, err)
// 	err = file.Close()
// 	require.NoError(t, err)
// 	name := file.Name()

// 	// open and change permissions
// 	af, err := OpenAutoFile(name)
// 	require.NoError(t, err)
// 	err = af.file.Chmod(0755)
// 	require.NoError(t, err)
// 	err = af.Close()
// 	require.NoError(t, err)

// 	// reopen and expect an ErrPermissionsChanged as Cause
// 	af, err = OpenAutoFile(name)
// 	require.Error(t, err)
// 	if e, ok := err.(*errors.ErrPermissionsChanged); ok {
// 		t.Logf("%v", e)
// 	} else {
// 		t.Errorf("unexpected error %v", e)
// 	}
// }

func TestAutoFileSize(t *testing.T) {
	// First, create an AutoFile writing to a tempfile dir
	f, err := os.CreateTemp("", "sighup_test")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Here is the actual AutoFile.
	af, err := OpenAutoFile(f.Name())
	require.NoError(t, err)

	// 1. Empty file
	size, err := af.Size()
	require.Zero(t, size)
	require.NoError(t, err)

	// 2. Not empty file
	data := []byte("Maniac\n")
	_, err = af.Write(data)
	require.NoError(t, err)
	size, err = af.Size()
	require.EqualValues(t, len(data), size)
	require.NoError(t, err)

	// 3. Not existing file
	require.NoError(t, af.Close())
	require.NoError(t, os.Remove(f.Name()))
	size, err = af.Size()
	require.EqualValues(t, 0, size, "Expected a new file to be empty")
	require.NoError(t, err)

	// Cleanup
	t.Cleanup(func() { os.Remove(f.Name()) })
}

func mustReadFile(t *testing.T, filePath string) []byte {
	fileBytes, err := os.ReadFile(filePath)
	require.NoError(t, err)

	return fileBytes
}

func TestAutoFileDeletionAndRecreation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFilePath := filepath.Join(tmpDir, "moveable")

	af, err := OpenAutoFile(tmpFilePath)
	require.Nil(t, err)
	defer func() {
		af.Close()
		require.Nil(t, os.RemoveAll(tmpFilePath), "Clean up failed")
	}()

	// Firstly reset the ticker so that it doesn't fire and that we are in absolute
	// control of syncFile and others.
	af.refreshTicker.Reset(time.Hour)

	intro := []byte("Hello, autofile!")
	n, err := af.Write(intro)
	require.Nil(t, err)
	require.Equal(t, len(intro), n, "Expecting a match in the number of bytes written")
	require.Nil(t, af.syncFile(), "syncFile should succeed")

	// Explicitly invoke syncFile.
	require.Nil(t, af.syncFile(), "syncFile should succeed")
	require.NotNil(t, af.file, "file should be non-nil")

	// Ensure that the file contents our content.
	readBlob := mustReadFile(t, tmpFilePath)
	require.Equal(t, intro, readBlob, "Mismatch of the file")

	// Now delete the file.
	require.Nil(t, os.RemoveAll(tmpFilePath), "Expected the file removal to succeed")
	fi, err := os.Lstat(tmpFilePath)
	require.NotNil(t, err, "Expecting the file not to exist")
	require.True(t, os.IsNotExist(err), "The error should report non-existence")
	require.Nil(t, fi)

	// Invoke a .Write right now, then ensure that that write
	// triggered a recreation of the file.
	ni, err := af.Write(intro)
	require.Nil(t, err)
	require.Equal(t, len(intro), ni, "Expecting a match in the number of bytes written")
	require.Nil(t, af.syncFile(), "syncFile should succeed")
	reReadBlob := mustReadFile(t, tmpFilePath)
	require.Equal(t, intro, reReadBlob, "Mismatch of the file")
}
