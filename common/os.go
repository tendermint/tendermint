package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
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

func EnsureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0700)
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

func WriteFile(filePath string, contents []byte) error {
	err := ioutil.WriteFile(filePath, contents, 0600)
	if err != nil {
		return err
	}
	fmt.Printf("File written to %v.\n", filePath)
	return nil
}

func MustWriteFile(filePath string, contents []byte) {
	err := WriteFile(filePath, contents)
	if err != nil {
		Exit(Fmt("MustWriteFile failed: %v", err))
	}
}

// Writes to newBytes to filePath.
// Guaranteed not to lose *both* oldBytes and newBytes,
// (assuming that the OS is perfect)
func WriteFileAtomic(filePath string, newBytes []byte) error {
	// If a file already exists there, copy to filePath+".bak" (overwrite anything)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("Could not read file %v. %v", filePath, err)
		}
		err = ioutil.WriteFile(filePath+".bak", fileBytes, 0600)
		if err != nil {
			return fmt.Errorf("Could not write file %v. %v", filePath+".bak", err)
		}
	}
	// Write newBytes to filePath.new
	err := ioutil.WriteFile(filePath+".new", newBytes, 0600)
	if err != nil {
		return fmt.Errorf("Could not write file %v. %v", filePath+".new", err)
	}
	// Move filePath.new to filePath
	err = os.Rename(filePath+".new", filePath)
	return err
}
