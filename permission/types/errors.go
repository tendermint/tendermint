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

// unknown string for permission
type ErrInvalidPermissionString string

func (e ErrInvalidPermissionString) Error() string {
	return fmt.Sprintf("invalid permission '%s'", e)
}

// already exists (err on add)
type ErrPermissionExists string

func (e ErrPermissionExists) Error() string {
	return fmt.Sprintf("permission '%s' already exists", e)
}

// unknown string for snative contract
type ErrInvalidSNativeString string

func (e ErrInvalidSNativeString) Error() string {
	return fmt.Sprintf("invalid snative contract '%s'", e)
}

// set=false. This error should be caught and the global
// value fetched for the permission by the caller
type ErrValueNotSet PermFlag

func (e ErrValueNotSet) Error() string {
	return fmt.Sprintf("the value for permission %d is not set", e)
}
