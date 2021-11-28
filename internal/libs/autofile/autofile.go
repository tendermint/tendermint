package autofile

import (
	"errors"
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
	panic(err)
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
	panic(err)
}
*/

const (
	autoFileClosePeriod = 1000 * time.Millisecond
	autoFilePerms       = os.FileMode(0600)
)

// AutoFile handles persisting data to a target file every 1 second. If the file was
// perhaps deleted, it'll re-create it. On receiving SIGHUP, it'll automatically close.
//
// This is useful for using a log file with the logrotate tool.
type AutoFile struct {
	ID   string
	Path string

	refreshTicker    *time.Ticker
	closeTickerStopc chan struct{} // closed when closeTicker is stopped
	hupc             chan os.Signal

	mtx  sync.Mutex
	file *os.File
}

// OpenAutoFile creates an AutoFile in the path (with random ID). If there is
// an error, it will be of type *PathError or *ErrPermissionsChanged (if file's
// permissions got changed (should be 0600)).
func OpenAutoFile(path string) (*AutoFile, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	af := &AutoFile{
		ID:               tmrand.Str(12) + ":" + path,
		Path:             path,
		refreshTicker:    time.NewTicker(autoFileClosePeriod),
		closeTickerStopc: make(chan struct{}),
	}
	if err := af.openFile(); err != nil {
		af.Close()
		return nil, err
	}

	// Close file on SIGHUP.
	af.hupc = make(chan os.Signal, 1)
	signal.Notify(af.hupc, syscall.SIGHUP)
	go func() {
		for range af.hupc {
			_ = af.closeFile()
		}
	}()

	go af.syncFileRoutine()

	return af, nil
}

// Close shuts down the closing goroutine, SIGHUP handler and closes the
// AutoFile.
func (af *AutoFile) Close() error {
	af.refreshTicker.Stop()
	close(af.closeTickerStopc)
	if af.hupc != nil {
		close(af.hupc)
	}
	return af.closeFile()
}

func (af *AutoFile) syncFileRoutine() {
	for {
		select {
		case <-af.refreshTicker.C:
			_ = af.syncFile()
		case <-af.closeTickerStopc:
			return
		}
	}
}

func (af *AutoFile) syncFile() error {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		return nil
	}

	// Firstly let's try to stat to see if the file is still available.
	if fi, err := os.Lstat(af.Path); err != nil || fi.IsDir() {
		// The file disappeared so remove the reference,
		// and it shall be reopened at the next write.
		af.file = nil
		return nil
	}

	err := af.file.Sync()
	if err == nil {
		return nil
	}

	// There could be multiple reasons why the .Sync failed,
	// maybe the file permissions changed?
	_ = af.file.Close()

	// Otherwise, examine if we actually need to create a new file.
	if errors.Is(err, os.ErrInvalid) {
		// It shall be opened for the next write.
		af.file = nil
		return nil
	}

	return err
}

func (af *AutoFile) closeFile() (err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	file := af.file
	if file == nil {
		return nil
	}

	af.file = nil
	return file.Close()
}

// needsReOpen returns true if the underlying file
// is nil or if the file cannot be stat-ed.
func (af *AutoFile) needsReOpen() bool {
	if af.file == nil {
		return true
	}
	_, err := os.Lstat(af.Path)
	return err != nil
}

// Write writes len(b) bytes to the AutoFile. It returns the number of bytes
// written and an error, if any. Write returns a non-nil error when n !=
// len(b).
// Opens AutoFile if needed.
func (af *AutoFile) Write(b []byte) (n int, err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.needsReOpen() {
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
// Opens AutoFile if needed.
func (af *AutoFile) Sync() error {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		if err := af.openFile(); err != nil {
			return err
		}
	}
	return af.file.Sync()
}

func (af *AutoFile) openFile() error {
	file, err := os.OpenFile(af.Path, os.O_RDWR|os.O_CREATE|os.O_APPEND, autoFilePerms)
	if err != nil {
		return err
	}
	// fileInfo, err := file.Stat()
	// if err != nil {
	// 	return err
	// }
	// if fileInfo.Mode() != autoFilePerms {
	// 	return errors.NewErrPermissionsChanged(file.Name(), fileInfo.Mode(), autoFilePerms)
	// }
	af.file = file
	return nil
}

// Size returns the size of the AutoFile. It returns -1 and an error if fails
// get stats or open file.
// Opens AutoFile if needed.
func (af *AutoFile) Size() (int64, error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

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
