package state

// TODO: This logic is crude. Should be more transactional.

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"sync"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	. "github.com/tendermint/tendermint/consensus/types"

	"github.com/tendermint/go-ed25519"
)

const (
	stepNone      = 0 // Used to distinguish the initial state
	stepPropose   = 1
	stepPrevote   = 2
	stepPrecommit = 3
	stepCommit    = 4
)

func voteToStep(vote *Vote) uint8 {
	switch vote.Type {
	case VoteTypePrevote:
		return stepPrevote
	case VoteTypePrecommit:
		return stepPrecommit
	case VoteTypeCommit:
		return stepCommit
	default:
		panic("Unknown vote type")
	}
}

type PrivValidator struct {
	Address    []byte
	PubKey     PubKeyEd25519
	PrivKey    PrivKeyEd25519
	LastHeight uint
	LastRound  uint
	LastStep   uint8

	// For persistence.
	// Overloaded for testing.
	filename string
	mtx      sync.Mutex
}

// Generates a new validator with private key.
func GenPrivValidator() *PrivValidator {
	privKeyBytes := CRandBytes(32)
	pubKeyBytes := ed25519.MakePubKey(privKeyBytes)
	pubKey := PubKeyEd25519(pubKeyBytes)
	privKey := PrivKeyEd25519{pubKeyBytes, privKeyBytes}
	return &PrivValidator{
		Address:    pubKey.Address(),
		PubKey:     pubKey,
		PrivKey:    privKey,
		LastHeight: 0,
		LastRound:  0,
		LastStep:   stepNone,
		filename:   config.App.GetString("PrivValidatorFile"),
	}
}

func LoadPrivValidator(filename string) *PrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	privVal := ReadJSON(&PrivValidator{}, privValJSONBytes, &err).(*PrivValidator)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filename, err))
	}
	privVal.filename = filename
	return privVal
}

func (privVal *PrivValidator) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *PrivValidator) save() {
	jsonBytes := privVal.JSONBytes()
	err := ioutil.WriteFile(privVal.filename, jsonBytes, 0700)
	if err != nil {
		panic(err)
	}
}

func (privVal *PrivValidator) JSONBytes() []byte { return JSONBytes(privVal) }

// TODO: test
func (privVal *PrivValidator) SignVote(vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()

	// If height regression, panic
	if privVal.LastHeight > vote.Height {
		return errors.New("Height regression in SignVote")
	}
	// More cases for when the height matches
	if privVal.LastHeight == vote.Height {
		// If attempting any sign after commit, panic
		if privVal.LastStep == stepCommit {
			return errors.New("SignVote on matching height after a commit")
		}
		// If round regression, panic
		if privVal.LastRound > vote.Round {
			return errors.New("Round regression in SignVote")
		}
		// If step regression, panic
		if privVal.LastRound == vote.Round && privVal.LastStep > voteToStep(vote) {
			return errors.New("Step regression in SignVote")
		}
	}

	// Persist height/round/step
	privVal.LastHeight = vote.Height
	privVal.LastRound = vote.Round
	privVal.LastStep = voteToStep(vote)
	privVal.save()

	// Sign
	privVal.SignVoteUnsafe(vote)
	return nil
}

func (privVal *PrivValidator) SignVoteUnsafe(vote *Vote) {
	vote.Signature = privVal.PrivKey.Sign(SignBytes(vote)).(SignatureEd25519)
}

func (privVal *PrivValidator) SignProposal(proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	if privVal.LastHeight < proposal.Height ||
		privVal.LastHeight == proposal.Height && privVal.LastRound < proposal.Round ||
		privVal.LastHeight == 0 && privVal.LastRound == 0 && privVal.LastStep == stepNone {

		// Persist height/round/step
		privVal.LastHeight = proposal.Height
		privVal.LastRound = proposal.Round
		privVal.LastStep = stepPropose
		privVal.save()

		// Sign
		proposal.Signature = privVal.PrivKey.Sign(SignBytes(proposal)).(SignatureEd25519)
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of proposal: Height %v, Round %v", proposal.Height, proposal.Round))
	}
}

func (privVal *PrivValidator) SignRebondTx(rebondTx *RebondTx) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	if privVal.LastHeight < rebondTx.Height {

		// Persist height/round/step
		privVal.LastHeight = rebondTx.Height
		privVal.LastRound = math.MaxUint64 // We can't do anything else for this rebondTx.Height.
		privVal.LastStep = math.MaxUint8
		privVal.save()

		// Sign
		rebondTx.Signature = privVal.PrivKey.Sign(SignBytes(rebondTx)).(SignatureEd25519)
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of rebondTx: Height %v", rebondTx.Height))
	}
}

func (privVal *PrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%X LH:%v, LR:%v, LS:%v}", privVal.Address, privVal.LastHeight, privVal.LastRound, privVal.LastStep)
}
