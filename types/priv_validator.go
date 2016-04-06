package types

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sync"

	acm "github.com/eris-ltd/tendermint/account"
	. "github.com/eris-ltd/tendermint/common"
	"github.com/eris-ltd/tendermint/wire"

	"github.com/tendermint/ed25519"
)

const (
	stepNone      = 0 // Used to distinguish the initial state
	stepPropose   = 1
	stepPrevote   = 2
	stepPrecommit = 3
)

func voteToStep(vote *Vote) int8 {
	switch vote.Type {
	case VoteTypePrevote:
		return stepPrevote
	case VoteTypePrecommit:
		return stepPrecommit
	default:
		PanicSanity("Unknown vote type")
		return 0
	}
}

type PrivValidator struct {
	Address    []byte            `json:"address"`
	PubKey     acm.PubKeyEd25519 `json:"pub_key"`
	LastHeight int               `json:"last_height"`
	LastRound  int               `json:"last_round"`
	LastStep   int8              `json:"last_step"`

	// PrivKey should be empty if a Signer other than the default is being used.
	PrivKey acm.PrivKeyEd25519 `json:"priv_key"`
	signer  Signer

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

// XXX: This is used to sign votes.
// It is the caller's duty to verify the msg before calling Sign,
// eg. to avoid double signing.
type Signer interface {
	Sign(msg []byte) acm.SignatureEd25519
}

// Implements Signer
type DefaultSigner struct {
	priv acm.PrivKeyEd25519
}

func NewDefaultSigner(priv acm.PrivKeyEd25519) *DefaultSigner {
	return &DefaultSigner{priv: priv}
}

// Implements Signer
func (ds *DefaultSigner) Sign(msg []byte) acm.SignatureEd25519 {
	return ds.priv.Sign(msg).(acm.SignatureEd25519)
}

func (privVal *PrivValidator) SetSigner(s Signer) {
	privVal.signer = s
}

// Generates a new validator with private key.
func GenPrivValidator() *PrivValidator {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	pubKeyBytes := ed25519.MakePublicKey(privKeyBytes)
	pubKey := acm.PubKeyEd25519(*pubKeyBytes)
	privKey := acm.PrivKeyEd25519(*privKeyBytes)
	return &PrivValidator{
		Address:    pubKey.Address(),
		PubKey:     pubKey,
		PrivKey:    privKey,
		LastHeight: 0,
		LastRound:  0,
		LastStep:   stepNone,
		filePath:   "",
		signer:     NewDefaultSigner(privKey),
	}
}

func LoadPrivValidator(filePath string) *PrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := wire.ReadJSON(&PrivValidator{}, privValJSONBytes, &err).(*PrivValidator)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}
	privVal.filePath = filePath
	privVal.signer = NewDefaultSigner(privVal.PrivKey)
	return privVal
}

func LoadOrGenPrivValidator(filePath string) *PrivValidator {
	var privValidator *PrivValidator
	if _, err := os.Stat(filePath); err == nil {
		privValidator = LoadPrivValidator(filePath)
		log.Notice("Loaded PrivValidator",
			"file", filePath, "privValidator", privValidator)
	} else {
		privValidator = GenPrivValidator()
		privValidator.SetFile(filePath)
		privValidator.Save()
		log.Notice("Generated PrivValidator", "file", filePath)
	}
	return privValidator
}

func (privVal *PrivValidator) SetFile(filePath string) {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.filePath = filePath
}

func (privVal *PrivValidator) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *PrivValidator) save() {
	if privVal.filePath == "" {
		PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes := wire.JSONBytes(privVal)
	err := WriteFileAtomic(privVal.filePath, jsonBytes)
	if err != nil {
		// `@; BOOM!!!
		PanicCrisis(err)
	}
}

func (privVal *PrivValidator) SignVote(chainID string, vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()

	// If height regression, panic
	if privVal.LastHeight > vote.Height {
		return errors.New("Height regression in SignVote")
	}
	// More cases for when the height matches
	if privVal.LastHeight == vote.Height {
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
	vote.Signature = privVal.signer.Sign(acm.SignBytes(chainID, vote))
	return nil
}

func (privVal *PrivValidator) SignProposal(chainID string, proposal *Proposal) error {
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
		proposal.Signature = privVal.signer.Sign(acm.SignBytes(chainID, proposal))
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of proposal: Height %v, Round %v", proposal.Height, proposal.Round))
	}
}

func (privVal *PrivValidator) SignRebondTx(chainID string, rebondTx *RebondTx) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	if privVal.LastHeight < rebondTx.Height {

		// Persist height/round/step
		// Prevent doing anything else for this rebondTx.Height.
		privVal.LastHeight = rebondTx.Height
		privVal.LastRound = math.MaxInt32 // MaxInt64 overflows on 32bit architectures.
		privVal.LastStep = math.MaxInt8
		privVal.save()

		// Sign
		rebondTx.Signature = privVal.signer.Sign(acm.SignBytes(chainID, rebondTx))
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of rebondTx: Height %v", rebondTx.Height))
	}
}

func (privVal *PrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%X LH:%v, LR:%v, LS:%v}", privVal.Address, privVal.LastHeight, privVal.LastRound, privVal.LastStep)
}

//-------------------------------------

type PrivValidatorsByAddress []*PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address, pvs[j].Address) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
