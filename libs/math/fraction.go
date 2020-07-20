package math

import "fmt"

// Fraction defined in terms of a numerator divided by a denominator in int64
// format.
type Fraction struct {
	// The portion of the denominator in the faction, e.g. 2 in 2/3.
	Numerator int64 `json:"numerator"`
	// The value by which the numerator is divided, e.g. 3 in 2/3. Must be
	// positive.
	Denominator int64 `json:"denominator"`
}

func (fr Fraction) String() string {
	return fmt.Sprintf("%d/%d", fr.Numerator, fr.Denominator)
}
