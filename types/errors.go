package types

import "fmt"

type (
	// ErrInvalidCommitHeight is returned when we encounter a commit with an
	// unexpected height.
	ErrInvalidCommitHeight struct {
		Expected int64
		Actual   int64
	}

	// ErrInvalidCommitPrecommits is returned when we encounter a commit where
	// the number of precommits doesn't match the number of validators.
	ErrInvalidCommitPrecommits struct {
		Expected int
		Actual   int
	}
)

func NewErrInvalidCommitHeight(expected, actual int64) ErrInvalidCommitHeight {
	return ErrInvalidCommitHeight{
		Expected: expected,
		Actual:   actual,
	}
}

func (e ErrInvalidCommitHeight) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong height: %v vs %v", e.Expected, e.Actual)
}

func NewErrInvalidCommitPrecommits(expected, actual int) ErrInvalidCommitPrecommits {
	return ErrInvalidCommitPrecommits{
		Expected: expected,
		Actual:   actual,
	}
}

func (e ErrInvalidCommitPrecommits) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong set size: %v vs %v", e.Expected, e.Actual)
}
