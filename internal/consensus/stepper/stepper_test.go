package stepper_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/consensus/stepper"
	"github.com/tendermint/tendermint/internal/consensus/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestRunStep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wasCalled := false
	alwaysTrue := func(types.RoundState) bool {
		return true
	}
	emptyTransition := func(types.RoundState) (types.RoundState, consensus.Message) {
		wasCalled = true
		return types.RoundState{}, &consensus.VoteMessage{}
	}

	ops := []stepper.Operation{
		{
			P: alwaysTrue,
			T: emptyTransition,
		},
	}
	s := stepper.New(ops)
	_, _, err := s.Next(ctx, types.RoundState{})
	if err != nil {
		t.Fatalf("unexepected error from Next %v", err)
	}
	if !wasCalled {
		t.Fatal("expected transition to be call")
	}
}

var line22Predicate = func(s types.RoundState) bool {
	return s.Step == types.RoundStepPropose &&
		s.ProposalBlockParts.IsComplete() &&
		s.Proposal.POLRound == -1 &&
		s.Proposal.Round == s.Round &&
		s.Proposal.Height == s.Height &&
		bytes.Equal(s.Validators.GetProposer().Address, s.ProposalBlock.ProposerAddress)
}

var line22Transition = func(s types.RoundState) (types.RoundState, consensus.Message) {
	msg := &consensus.VoteMessage{
		Vote: &tmtypes.Vote{
			Height:  s.Height,
			Round:   s.Round,
			BlockID: tmtypes.BlockID{},
		},
	}
	if valid(s.ProposalBlock) && (s.LockedRound == -1 || s.LockedBlock != nil && s.LockedBlock.HashesTo(s.ProposalBlock.Hash())) {
		msg = &consensus.VoteMessage{
			Vote: &tmtypes.Vote{
				Height:  s.Height,
				Round:   s.Round,
				BlockID: tmtypes.BlockID{Hash: s.ProposalBlockParts.Hash(), PartSetHeader: s.ProposalBlockParts.Header()},
			},
		}
	}
	s.Step = types.RoundStepPrevote
	return s, msg
}

func valid(b *tmtypes.Block) bool {
	return true
}

func TestLine22(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ops := []stepper.Operation{
		{
			P: line22Predicate,
			T: line22Transition,
		},
	}
	s := stepper.New(ops)

	addr := []byte("proposer")
	r := types.RoundState{
		Round:       1,
		Height:      1,
		LockedRound: -1,
		Step:        types.RoundStepPropose,
		Proposal: &tmtypes.Proposal{
			POLRound: -1,
			Round:    1,
			Height:   1,
		},
		ProposalBlockParts: tmtypes.NewPartSetFromData([]byte("part set"), 5),
		ProposalBlock: &tmtypes.Block{
			Header: tmtypes.Header{
				ProposerAddress: addr,
			},
		},
		Validators: &tmtypes.ValidatorSet{
			Validators: []*tmtypes.Validator{
				{
					Address: addr,
				},
			},
			Proposer: &tmtypes.Validator{
				Address: addr,
			},
		},
	}

	r, msg, err := s.Next(ctx, r)
	if err != nil {
		t.Fatalf("unexepected error from Next %v", err)
	}
	if msg == nil {
		t.Fatalf("expected message to not be nil")
	}

	if vote, ok := msg.(*consensus.VoteMessage); ok {
		if vote.Vote.BlockID.IsNil() {
			t.Fatalf("expected vote to be for block")
		}
	} else {
		t.Fatalf("expected message to not be vote")
	}
	if r.Step != types.RoundStepPrevote {
		t.Fatalf("expected step to be %v but saw %v", types.RoundStepPrevote, r.Step)
	}
}
