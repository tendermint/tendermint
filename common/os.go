package common

import (
	"fmt"
	"io/ioutil"
	"os"
)

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

func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}
