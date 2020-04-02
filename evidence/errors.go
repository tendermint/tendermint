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

// ErrEvidenceAlreadyStored indicates that the evidence has already been stored in the evidence db
type ErrEvidenceAlreadyStored struct{}

func (e ErrEvidenceAlreadyStored) Error() string {
	return "evidence is already stored"
}
