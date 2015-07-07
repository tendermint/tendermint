package types

import (
	"fmt"
)

//------------------------------------------------------------------------------------------------
// Some errors

// permission number out of bounds
type ErrInvalidPermission PermFlag

func (e ErrInvalidPermission) Error() string {
	return fmt.Sprintf("invalid permission %d", e)
}

// set=false. This error should be caught and the global
// value fetched for the permission by the caller
type ErrValueNotSet PermFlag

func (e ErrValueNotSet) Error() string {
	return fmt.Sprintf("the value for permission %d is not set", e)
}
