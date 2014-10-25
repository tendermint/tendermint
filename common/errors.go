package common

import (
	"errors"
	"fmt"
)

func Errorf(s string, args ...interface{}) error {
	return errors.New(fmt.Sprintf(s, args...))
}

type StackError struct {
	Err   interface{}
	Stack []byte
}

func (se StackError) String() string {
	return fmt.Sprintf("Error: %v\nStack: %s", se.Err, se.Stack)
}

func (se StackError) Error() string {
	return se.String()
}
