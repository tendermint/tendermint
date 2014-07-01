package common

import (
	"fmt"
)

func Panicf(s string, args ...interface{}) {
	panic(fmt.Sprintf(s, args...))
}
