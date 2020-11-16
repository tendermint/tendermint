package os

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

type logger interface {
	Info(msg string, keyvals ...interface{})
}

// TrapSignal catches SIGTERM and SIGINT, executes the cleanup function,
// and exits with code 0.
func TrapSignal(logger logger, cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		logger.Info(fmt.Sprintf("captured %v, exiting...", sig))
		if cb != nil {
			cb()
		}
		os.Exit(0)
	}()
}

func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}

func EnsureDir(dir string, mode os.FileMode) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, mode)
		if err != nil {
			return fmt.Errorf("could not create directory %v: %w", dir, err)
		}
	}
	return nil
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// CopyFile copies a file. It truncates the destination file if it exists.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	// create new file, truncate if exists and apply same permissions as the original one
	dstfile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer dstfile.Close()

	_, err = io.Copy(dstfile, srcfile)
	return err
}
