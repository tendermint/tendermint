package common

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
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

func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return true, err //folder is non-existent
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
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

func Tempfile(prefix string) (*os.File, string) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		PanicCrisis(err)
	}
	return file, file.Name()
}

func Tempdir(prefix string) (*os.File, string) {
	tempDir := os.TempDir() + "/" + prefix + RandStr(12)
	err := EnsureDir(tempDir, 0700)
	if err != nil {
		panic(Fmt("Error creating temp dir: %v", err))
	}
	dir, err := os.Open(tempDir)
	if err != nil {
		panic(Fmt("Error opening temp dir: %v", err))
	}
	return dir, tempDir
}

//--------------------------------------------------------------------------------

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
