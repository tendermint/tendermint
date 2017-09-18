package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	. "github.com/tendermint/tmlibs/common"
)

// TODO: type ?
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

// Signer is an interface that defines how to sign votes.
// It is the caller's duty to verify the msg before calling Sign,
// eg. to avoid double signing.
// Currently, the only callers are SignVote and SignProposal.
type Signer interface {
	PubKey() crypto.PubKey
	Sign(msg []byte) (crypto.Signature, error)
}

// DefaultSigner implements Signer.
// It uses a standard crypto.PrivKey.
type DefaultSigner struct {
	PrivKey crypto.PrivKey `json:"priv_key"`
}

// NewDefaultSigner returns an instance of DefaultSigner.
func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner {
	return &DefaultSigner{PrivKey: priv}
}

// Sign implements Signer. It signs the byte slice with a private key.
func (ds *DefaultSigner) Sign(msg []byte) (crypto.Signature, error) {
	return ds.PrivKey.Sign(msg), nil
}

// PubKey implements Signer. It should return the public key that corresponds
// to the private key used for signing.
func (ds *DefaultSigner) PubKey() crypto.PubKey {
	return ds.PrivKey.PubKey()
}

type PrivValidator interface {
	Address() data.Bytes // redundant since .PubKey().Address()
	PubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error

	Reset()

	SetFile(file string)
	Save()
}

// DefaultPrivValidator implements the functionality for signing blocks.
type DefaultPrivValidator struct {
	Info   PrivValidatorInfo `json:"info"`
	Signer *DefaultSigner    `json:"signer"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

func (pv *DefaultPrivValidator) Address() data.Bytes {
	return pv.Info.Address
}

func (pv *DefaultPrivValidator) PubKey() crypto.PubKey {
	return pv.Info.PubKey
}

type PrivValidatorInfo struct {
	Address       data.Bytes       `json:"address"`
	PubKey        crypto.PubKey    `json:"pub_key"`
	LastHeight    int              `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes data.Bytes       `json:"last_signbytes,omitempty"` // so we dont lose signatures
}

func LoadOrGenPrivValidator(filePath string) *DefaultPrivValidator {
	var privValidator *DefaultPrivValidator
	if _, err := os.Stat(filePath); err == nil {
		privValidator = LoadPrivValidator(filePath)
	} else {
		privValidator = GenPrivValidator()
		privValidator.SetFile(filePath)
		privValidator.Save()
	}
	return privValidator
}

func LoadPrivValidator(filePath string) *DefaultPrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := DefaultPrivValidator{}
	err = json.Unmarshal(privValJSONBytes, &privVal)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	privVal.filePath = filePath
	return &privVal
}

// Generates a new validator with private key.
func GenPrivValidator() *DefaultPrivValidator {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	pubKey := privKey.PubKey()
	return &DefaultPrivValidator{
		Info: PrivValidatorInfo{
			Address:  pubKey.Address(),
			PubKey:   pubKey,
			LastStep: stepNone,
		},
		Signer:   NewDefaultSigner(privKey),
		filePath: "",
	}
}

// LoadPrivValidatorWithSigner instantiates a private validator with a custom
// signer object. Tendermint tracks state in the PrivValidator that might be
// saved to disk. Please supply a filepath where Tendermint can save the
// private validator.
func LoadPrivValidatorWithSigner(signer *DefaultSigner, filePath string) *DefaultPrivValidator {
	return &DefaultPrivValidator{
		Info: PrivValidatorInfo{
			Address:  signer.PubKey().Address(),
			PubKey:   signer.PubKey(),
			LastStep: stepNone,
		},
		Signer:   signer,
		filePath: filePath,
	}
}

// Overwrite address and pubkey for convenience
/*func (privVal *DefaultPrivValidator) setPubKeyAndAddress() {
	privVal.PubKey = privVal.Signer.PubKey()
	privVal.Address = privVal.PubKey.Address()
}*/

func (privVal *DefaultPrivValidator) SetFile(filePath string) {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.filePath = filePath
}

func (privVal *DefaultPrivValidator) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *DefaultPrivValidator) save() {
	if privVal.filePath == "" {
		PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(privVal)
	if err != nil {
		// `@; BOOM!!!
		PanicCrisis(err)
	}
	err = WriteFileAtomic(privVal.filePath, jsonBytes, 0600)
	if err != nil {
		// `@; BOOM!!!
		PanicCrisis(err)
	}
}

// NOTE: Unsafe!
func (privVal *DefaultPrivValidator) Reset() {
	privVal.Info.LastHeight = 0
	privVal.Info.LastRound = 0
	privVal.Info.LastStep = 0
	privVal.Info.LastSignature = crypto.Signature{}
	privVal.Info.LastSignBytes = nil
	privVal.Save()
}

func (privVal *DefaultPrivValidator) GetAddress() []byte {
	return privVal.Address()
}

func (privVal *DefaultPrivValidator) SignVote(chainID string, vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(vote.Height, vote.Round, voteToStep(vote), SignBytes(chainID, vote))
	if err != nil {
		return errors.New(Fmt("Error signing vote: %v", err))
	}
	vote.Signature = signature
	return nil
}

func (privVal *DefaultPrivValidator) SignProposal(chainID string, proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(proposal.Height, proposal.Round, stepPropose, SignBytes(chainID, proposal))
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	proposal.Signature = signature
	return nil
}

// check if there's a regression. Else sign and write the hrs+signature to disk
func (privVal *DefaultPrivValidator) signBytesHRS(height, round int, step int8, signBytes []byte) (crypto.Signature, error) {
	sig := crypto.Signature{}
	info := privVal.Info
	// If height regression, err
	if info.LastHeight > height {
		return sig, errors.New("Height regression")
	}
	// More cases for when the height matches
	if info.LastHeight == height {
		// If round regression, err
		if info.LastRound > round {
			return sig, errors.New("Round regression")
		}
		// If step regression, err
		if info.LastRound == round {
			if info.LastStep > step {
				return sig, errors.New("Step regression")
			} else if info.LastStep == step {
				if info.LastSignBytes != nil {
					if info.LastSignature.Empty() {
						PanicSanity("privVal: LastSignature is nil but LastSignBytes is not!")
					}
					// so we dont sign a conflicting vote or proposal
					// NOTE: proposals are non-deterministic (include time),
					// so we can actually lose them, but will still never sign conflicting ones
					if bytes.Equal(info.LastSignBytes, signBytes) {
						// log.Notice("Using info.LastSignature", "sig", info.LastSignature)
						return info.LastSignature, nil
					}
				}
				return sig, errors.New("Step regression")
			}
		}
	}

	// Sign
	sig, err := privVal.Signer.Sign(signBytes)
	if err != nil {
		return sig, err
	}

	// Persist height/round/step
	privVal.Info.LastHeight = height
	privVal.Info.LastRound = round
	privVal.Info.LastStep = step
	privVal.Info.LastSignature = sig
	privVal.Info.LastSignBytes = signBytes
	privVal.save()

	return sig, nil

}

func (privVal *DefaultPrivValidator) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	var err error
	heartbeat.Signature, err = privVal.Signer.Sign(SignBytes(chainID, heartbeat))
	return err
}

func (privVal *DefaultPrivValidator) String() string {
	info := privVal.Info
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", info.Address, info.LastHeight, info.LastRound, info.LastStep)
}

//-------------------------------------

type PrivValidatorsByAddress []*DefaultPrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Info.Address, pvs[j].Info.Address) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
