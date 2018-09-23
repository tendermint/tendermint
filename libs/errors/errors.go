// Package errors contains errors that are thrown across packages.
package errors

import (
	"fmt"
	"os"
)

// ErrPermissionsChanged occurs if the file permission have changed since the file was created.
type ErrPermissionsChanged struct {
	name      string
	got, want os.FileMode
}

func NewErrPermissionsChanged(name string, got, want os.FileMode) *ErrPermissionsChanged {
	return &ErrPermissionsChanged{name: name, got: got, want: want}
}

func (e ErrPermissionsChanged) Error() string {
	return fmt.Sprintf(
		"file: [%v]\nexpected file permissions: %v, got: %v",
		e.name,
		e.want,
		e.got,
	)
}
