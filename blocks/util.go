package blocks

import (
    "fmt"
)

func panicf(s string, args ...interface{}) {
    panic(fmt.Sprintf(s, args...))
}
