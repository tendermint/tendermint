package common

import (
    "fmt"
)

func panicf(s string, args ...interface{}) {
    panic(fmt.Sprintf(s, args...))
}
