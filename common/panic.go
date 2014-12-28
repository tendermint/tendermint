package common

import (
	"fmt"
	"os"
)

func Panicf(s string, args ...interface{}) {
	panic(fmt.Sprintf(s, args...))
}

func Exitf(s string, args ...interface{}) {
	fmt.Printf(s+"\n", args...)
	os.Exit(1)
}
