package common

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

var (
	GoPath = os.Getenv("GOPATH")
)

func TrapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)
	go func() {
		for sig := range c {
			fmt.Printf("captured %v, exiting...\n", sig)
			if cb != nil {
				cb()
			}
			os.Exit(1)
		}
	}()
	select {}
}

func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}

func EnsureDir(dir string, mode os.FileMode) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, mode)
		if err != nil {
			return fmt.Errorf("Could not create directory %v. %v", dir, err)
		}
	}
	return nil
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func ReadFile(filePath string) ([]byte, error) {
	return ioutil.ReadFile(filePath)
}

func MustReadFile(filePath string) []byte {
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(Fmt("MustReadFile failed: %v", err))
		return nil
	}
	return fileBytes
}

func WriteFile(filePath string, contents []byte, mode os.FileMode) error {
	err := ioutil.WriteFile(filePath, contents, mode)
	if err != nil {
		return err
	}
	// fmt.Printf("File written to %v.\n", filePath)
	return nil
}

func MustWriteFile(filePath string, contents []byte, mode os.FileMode) {
	err := WriteFile(filePath, contents, mode)
	if err != nil {
		Exit(Fmt("MustWriteFile failed: %v", err))
	}
}

// Writes to newBytes to filePath.
// Guaranteed not to lose *both* oldBytes and newBytes,
// (assuming that the OS is perfect)
func WriteFileAtomic(filePath string, newBytes []byte, mode os.FileMode) error {
	// If a file already exists there, copy to filePath+".bak" (overwrite anything)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("Could not read file %v. %v", filePath, err)
		}
		err = ioutil.WriteFile(filePath+".bak", fileBytes, mode)
		if err != nil {
			return fmt.Errorf("Could not write file %v. %v", filePath+".bak", err)
		}
	}
	// Write newBytes to filePath.new
	err := ioutil.WriteFile(filePath+".new", newBytes, mode)
	if err != nil {
		return fmt.Errorf("Could not write file %v. %v", filePath+".new", err)
	}
	// Move filePath.new to filePath
	err = os.Rename(filePath+".new", filePath)
	return err
}

//--------------------------------------------------------------------------------

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

const autoFileOpenDuration = 1000 * time.Millisecond

// Automatically closes and re-opens file for writing.
// This is useful for using a log file with the logrotate tool.
type AutoFile struct {
	Path   string
	ticker *time.Ticker
	mtx    sync.Mutex
	file   *os.File
}

func OpenAutoFile(path string) (af *AutoFile, err error) {
	af = &AutoFile{
		Path:   path,
		ticker: time.NewTicker(autoFileOpenDuration),
	}
	if err = af.openFile(); err != nil {
		return
	}
	go af.processTicks()
	return
}

func (af *AutoFile) Close() error {
	af.ticker.Stop()
	af.mtx.Lock()
	err := af.closeFile()
	af.mtx.Unlock()
	return err
}

func (af *AutoFile) processTicks() {
	for {
		_, ok := <-af.ticker.C
		if !ok {
			return // Done.
		}
		af.mtx.Lock()
		af.closeFile()
		af.mtx.Unlock()
	}
}

func (af *AutoFile) closeFile() (err error) {
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
	return af.file.Write(b)
}

func (af *AutoFile) openFile() error {
	file, err := os.OpenFile(af.Path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	af.file = file
	return nil
}

func Tempfile(prefix string) (*os.File, string) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		PanicCrisis(err)
	}
	return file, file.Name()
}

func Prompt(prompt string, defaultValue string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue, err
	} else {
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultValue, nil
		}
		return line, nil
	}
}
