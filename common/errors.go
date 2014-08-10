package common

import (
	"errors"
	"fmt"
)

func Errorf(s string, args ...interface{}) error {
	return errors.New(fmt.Sprintf(s, args...))
}
