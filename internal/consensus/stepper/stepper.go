package stepper

import (
	"context"
	"errors"

	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/consensus/types"
)

var (
	ErrNoValidTransition = errors.New("no valid transition")
)

var emptyTransition = func(types.RoundState) (types.RoundState, consensus.Message) {
	return types.RoundState{}, &consensus.VoteMessage{}
}

type (
	Transition func(types.RoundState) (types.RoundState, consensus.Message)
	Predicate  func(types.RoundState) bool
)

type Operation struct {
	Name string
	P    Predicate
	T    Transition
}

type stepper struct {
	ops []Operation
}

func New(ops []Operation) stepper {
	return stepper{ops: ops}
}

func (s *stepper) Next(ctx context.Context, state types.RoundState) (types.RoundState, consensus.Message, error) {
	if t, ok := s.pickTransition(state); ok {
		s, msg := t(state)
		return s, msg, nil
	}
	return types.RoundState{}, nil, ErrNoValidTransition
}

func (s *stepper) pickTransition(state types.RoundState) (Transition, bool) {
	for _, op := range s.ops {
		if op.P(state) {
			return op.T, true
		}
	}
	return emptyTransition, false
}
