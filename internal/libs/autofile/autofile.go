package autofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

/* AutoFile usage

// Create/Append to ./autofile_test
af, err := OpenAutoFile("autofile_test")
if err != nil {
        log.Fatal(err)
}

// Stream of writes.
// During this time, the file may be moved e.g. by logRotate.
for i := 0; i < 60; i++ {
	af.Write([]byte(Fmt("LOOP(%v)", i)))
	time.Sleep(time.Second)
}

// Close the AutoFile
err = af.Close()
if err != nil {
	log.Fatal(err)
}
*/

const (
	autoFileClosePeriod = 1000 * time.Millisecond
	autoFilePerms       = os.FileMode(0600)
)

// ErrAutoFileClosed is reported when operations attempt to use an autofile
// after it has been closed.
var ErrAutoFileClosed = errors.New("autofile is closed")

// AutoFile automatically closes and re-opens file for writing. The file is
// automatically setup to close itself every 1s and upon receiving SIGHUP.
//
// This is useful for using a log file with the logrotate tool.
type AutoFile struct {
	ID   string
	Path string

	closeTicker *time.Ticker // signals periodic close
	cancel      func()       // cancels the lifecycle context

	mtx    sync.Mutex // guards the fields below
	closed bool       // true when the the autofile is no longer usable
	file   *os.File   // the underlying file (may be nil)
}

// OpenAutoFile creates an AutoFile in the path (with random ID). If there is
// an error, it will be of type *PathError or *ErrPermissionsChanged (if file's
// permissions got changed (should be 0600)).
func OpenAutoFile(ctx context.Context, path string) (*AutoFile, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	af := &AutoFile{
		ID:          tmrand.Str(12) + ":" + path,
		Path:        path,
		closeTicker: time.NewTicker(autoFileClosePeriod),
		cancel:      cancel,
	}
	if err := af.openFile(); err != nil {
		af.Close()
		return nil, err
	}

	// Set up a SIGHUP handler to forcibly flush and close the filehandle.
	// This forces the next operation to re-open the underlying path.
	hupc := make(chan os.Signal, 1)
	signal.Notify(hupc, syscall.SIGHUP)
	go func() {
		defer close(hupc)
		for {
			select {
			case <-hupc:
				_ = af.closeFile()
			case <-ctx.Done():
				return
			}
		}
	}()

	go af.closeFileRoutine(ctx)

	return af, nil
}

// Close shuts down the service goroutine and marks af as invalid.  Operations
// on af after Close will report an error.
func (af *AutoFile) Close() error {
	return af.withLock(func() error {
		af.cancel()      // signal the close service to stop
		af.closed = true // mark the file as invalid
		return af.unsyncCloseFile()
	})
}

func (af *AutoFile) closeFileRoutine(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			_ = af.Close()
			return
		case <-af.closeTicker.C:
			_ = af.closeFile()
		}
	}
}

func (af *AutoFile) closeFile() (err error) {
	return af.withLock(af.unsyncCloseFile)
}

// unsyncCloseFile closes the underlying filehandle if one is open, and reports
// any error it returns. The caller must hold af.mtx exclusively.
func (af *AutoFile) unsyncCloseFile() error {
	if fp := af.file; fp != nil {
		af.file = nil
		return fp.Close()
	}
	return nil
}

// withLock runs f while holding af.mtx, and reports any error it returns.
func (af *AutoFile) withLock(f func() error) error {
	af.mtx.Lock()
	defer af.mtx.Unlock()
	return f()
}

// Write writes len(b) bytes to the AutoFile. It returns the number of bytes
// written and an error, if any. Write returns a non-nil error when n !=
// len(b).
// Opens AutoFile if needed.
func (af *AutoFile) Write(b []byte) (n int, err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()
	if af.closed {
		return 0, fmt.Errorf("write: %w", ErrAutoFileClosed)
	}

	if af.file == nil {
		if err = af.openFile(); err != nil {
			return
		}
	}

	n, err = af.file.Write(b)
	return
}

// Sync commits the current contents of the file to stable storage. Typically,
// this means flushing the file system's in-memory copy of recently written
// data to disk.
func (af *AutoFile) Sync() error {
	return af.withLock(func() error {
		if af.closed {
			return fmt.Errorf("sync: %w", ErrAutoFileClosed)
		} else if af.file == nil {
			return nil // nothing to sync
		}
		return af.file.Sync()
	})
}

// openFile unconditionally replaces af.file with a new filehandle on the path.
// The caller must hold af.mtx exclusively.
func (af *AutoFile) openFile() error {
	file, err := os.OpenFile(af.Path, os.O_RDWR|os.O_CREATE|os.O_APPEND, autoFilePerms)
	if err != nil {
		return err
	}

	af.file = file
	return nil
}

// Size returns the size of the AutoFile. It returns -1 and an error if fails
// get stats or open file.
// Opens AutoFile if needed.
func (af *AutoFile) Size() (int64, error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()
	if af.closed {
		return 0, fmt.Errorf("size: %w", ErrAutoFileClosed)
	}

	if af.file == nil {
		if err := af.openFile(); err != nil {
			return -1, err
		}
	}

	stat, err := af.file.Stat()
	if err != nil {
		return -1, err
	}
	return stat.Size(), nil
}
