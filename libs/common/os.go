package common

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

var gopath string

// GoPath returns GOPATH env variable value. If it is not set, this function
// will try to call `go env GOPATH` subcommand.
func GoPath() string {
	if gopath != "" {
		return gopath
	}

	path := os.Getenv("GOPATH")
	if len(path) == 0 {
		goCmd := exec.Command("go", "env", "GOPATH")
		out, err := goCmd.Output()
		if err != nil {
			panic(fmt.Sprintf("failed to determine gopath: %v", err))
		}
		path = string(out)
	}
	gopath = path
	return path
}

// TrapSignal catches the SIGTERM and executes cb function. After that it exits
// with code 1.
func TrapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
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

// Kill the running process by sending itself SIGTERM.
func Kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
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
		if os.IsNotExist(err) {
			return true, err
		}
		// Otherwise perhaps a permission
		// error or some other error.
		return false, err
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
		Exit(fmt.Sprintf("MustReadFile failed: %v", err))
		return nil
	}
	return fileBytes
}

func WriteFile(filePath string, contents []byte, mode os.FileMode) error {
	return ioutil.WriteFile(filePath, contents, mode)
}

func MustWriteFile(filePath string, contents []byte, mode os.FileMode) {
	err := WriteFile(filePath, contents, mode)
	if err != nil {
		Exit(fmt.Sprintf("MustWriteFile failed: %v", err))
	}
}

//--------------------------------------------------------------------------------

func Prompt(prompt string, defaultValue string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}
