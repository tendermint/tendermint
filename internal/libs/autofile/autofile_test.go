package autofile

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSIGHUP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Error(err)
		}
	})

	// First, create a temporary directory and move into it
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))

	// Create an AutoFile in the temporary directory
	name := "sighup_test"
	af, err := OpenAutoFile(ctx, name)
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
	otherDir := t.TempDir()
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First, create an AutoFile writing to a tempfile dir
	f, err := os.CreateTemp(t.TempDir(), "sighup_test")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Here is the actual AutoFile.
	af, err := OpenAutoFile(ctx, f.Name())
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
	require.NoError(t, af.closeFile())
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
