package evidence

import (
	"fmt"
)

// ErrInvalidEvidence returns when evidence failed to validate
type ErrInvalidEvidence struct {
	Reason error
}

func (e ErrInvalidEvidence) Error() string {
	return fmt.Sprintf("evidence is not valid: %v ", e.Reason)
}

type ErrDatabase struct {
	DBErr error
}

func (e ErrDatabase) Error() string {
	return fmt.Sprintf("database error: %v", e.DBErr)
}
