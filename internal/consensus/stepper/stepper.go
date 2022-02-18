package stepper

import (
	"errors"

	"github.com/tendermint/tendermint/internal/consensus/types"
)

var (
	ErrNoValidTransition = errors.New("no valid transition")
)

type (
	Transition func(types.RoundState) types.RoundState
	Predicate  func(types.RoundState) bool
)

var emptyTransition = func(types.RoundState) types.RoundState { return types.RoundState{} }

type Operation struct {
	P Predicate
	T Transition
}

type stepper struct {
	ops []Operation
}

func New(ops []Operation) stepper {
	return stepper{ops: ops}
}

func (s *stepper) Next(state types.RoundState) (types.RoundState, error) {
	if t, ok := s.pickTransition(state); ok {
		return t(state), nil
	}
	return types.RoundState{}, ErrNoValidTransition
}

func (s *stepper) pickTransition(state types.RoundState) (Transition, bool) {
	for _, op := range s.ops {
		return op.T, true
	}
	return emptyTransition, false
}
