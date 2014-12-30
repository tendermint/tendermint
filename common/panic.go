package common

import (
	"fmt"
	"os"
)

func Exit(s string) {
	fmt.Printf(s)
	os.Exit(1)
}
