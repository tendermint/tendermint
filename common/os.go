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

// Writes to newBytes to filePath.
// Guaranteed not to lose *both* oldBytes and newBytes,
// (assuming that the OS is perfect)
func AtomicWriteFile(filePath string, newBytes []byte) error {
	// If a file already exists there, copy to filePath+".bak" (overwrite anything)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("Failed to read file %v. %v", filePath, err)
		}
		err = ioutil.WriteFile(filePath+".bak", fileBytes, 0600)
		if err != nil {
			return fmt.Errorf("Failed to write file %v. %v", filePath+".bak", err)
		}
	}
	// Write newBytes to filePath.
	err := ioutil.WriteFile(filePath, newBytes, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write file %v. %v", filePath, err)
	}
	return nil
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
