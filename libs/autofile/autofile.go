package autofile

import (
	"os"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/errors"
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
	autoFileOpenDuration = 1000 * time.Millisecond
	autoFilePerms        = os.FileMode(0600)
)

// Automatically closes and re-opens file for writing.
// This is useful for using a log file with the logrotate tool.
type AutoFile struct {
	ID            string
	Path          string
	ticker        *time.Ticker
	tickerStopped chan struct{} // closed when ticker is stopped
	mtx           sync.Mutex
	file          *os.File
}

func OpenAutoFile(path string) (af *AutoFile, err error) {
	af = &AutoFile{
		ID:            cmn.RandStr(12) + ":" + path,
		Path:          path,
		ticker:        time.NewTicker(autoFileOpenDuration),
		tickerStopped: make(chan struct{}),
	}
	if err = af.openFile(); err != nil {
		return
	}
	go af.processTicks()
	sighupWatchers.addAutoFile(af)
	return
}

func (af *AutoFile) Close() error {
	af.ticker.Stop()
	close(af.tickerStopped)
	err := af.closeFile()
	sighupWatchers.removeAutoFile(af)
	return err
}

func (af *AutoFile) processTicks() {
	for {
		select {
		case <-af.ticker.C:
			af.closeFile()
		case <-af.tickerStopped:
			return
		}
	}
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

func (af *AutoFile) Write(b []byte) (n int, err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		if err = af.openFile(); err != nil {
			return
		}
	}

	n, err = af.file.Write(b)
	return
}

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
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if fileInfo.Mode() != autoFilePerms {
		return errors.NewErrPermissionsChanged(file.Name(), fileInfo.Mode(), autoFilePerms)
	}
	af.file = file
	return nil
}

func (af *AutoFile) Size() (int64, error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		err := af.openFile()
		if err != nil {
			if err == os.ErrNotExist {
				return 0, nil
			}
			return -1, err
		}
	}
	stat, err := af.file.Stat()
	if err != nil {
		return -1, err
	}
	return stat.Size(), nil

}
