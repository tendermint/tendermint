package autofile

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

func init() {
	initSighupWatcher()
}

var sighupWatchers *SighupWatcher
var sighupCounter int32 // For testing

func initSighupWatcher() {
	sighupWatchers = newSighupWatcher()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for _ = range c {
			sighupWatchers.closeAll()
			atomic.AddInt32(&sighupCounter, 1)
		}
	}()
}

// Watchces for SIGHUP events and notifies registered AutoFiles
type SighupWatcher struct {
	mtx       sync.Mutex
	autoFiles map[string]*AutoFile
}

func newSighupWatcher() *SighupWatcher {
	return &SighupWatcher{
		autoFiles: make(map[string]*AutoFile, 10),
	}
}

func (w *SighupWatcher) addAutoFile(af *AutoFile) {
	w.mtx.Lock()
	w.autoFiles[af.ID] = af
	w.mtx.Unlock()
}

// If AutoFile isn't registered or was already removed, does nothing.
func (w *SighupWatcher) removeAutoFile(af *AutoFile) {
	w.mtx.Lock()
	delete(w.autoFiles, af.ID)
	w.mtx.Unlock()
}

func (w *SighupWatcher) closeAll() {
	w.mtx.Lock()
	for _, af := range w.autoFiles {
		af.closeFile()
	}
	w.mtx.Unlock()
}
