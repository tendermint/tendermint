package math

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

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

func NewFraction(f string) (Fraction, error) {
	o := strings.SplitN(f, "/", -1)
	if len(o) != 2 {
		return Fraction{}, errors.New("incorrect formating: should be like \"1/3\"")
	}
	numerator, err := strconv.ParseInt(o[0], 10, 64)
	if err != nil {
		return Fraction{}, fmt.Errorf("incorrect formatting, err: %w", err)
	}

	denominator, err := strconv.ParseInt(o[1], 10, 64)
	if err != nil {
		return Fraction{}, fmt.Errorf("incorrect formatting, err: %w", err)
	}
	return Fraction{Numerator: numerator, Denominator: denominator}, nil
}
