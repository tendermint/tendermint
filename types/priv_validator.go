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

// PrivValidator defines the functionality of a local Tendermint validator.
type PrivValidator interface {
	Address() data.Bytes // redundant since .PubKey().Address()
	PubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

// PrivValidatorFS implements PrivValidator using data persisted to disk
// to prevent double signing. The Signer itself can be mutated to use
// something besides the default, for instance a hardware signer.
type PrivValidatorFS struct {
	ID     ValidatorID `json:"id"`
	Signer Signer      `json:"signer"`

	// mutable state to be persisted to disk
	// after each signature to prevent double signing
	mtx  sync.Mutex
	Info LastSignedInfo `json:"info"`

	// For persistence.
	// Overloaded for testing.
	filePath string
}

// LoadOrGenPrivValidatorFS loads a PrivValidatorFS from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenPrivValidatorFS(filePath string) *PrivValidatorFS {
	var PrivValidatorFS *PrivValidatorFS
	if _, err := os.Stat(filePath); err == nil {
		PrivValidatorFS = LoadPrivValidatorFS(filePath)
	} else {
		PrivValidatorFS = GenPrivValidatorFS(filePath)
		PrivValidatorFS.Save()
	}
	return PrivValidatorFS
}

// LoadPrivValidatorFS loads a PrivValidatorFS from the filePath.
func LoadPrivValidatorFS(filePath string) *PrivValidatorFS {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := PrivValidatorFS{}
	err = json.Unmarshal(privValJSONBytes, &privVal)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	privVal.filePath = filePath
	return &privVal
}

// GenPrivValidatorFS generates a new validator with randomly generated private key
// and sets the filePath, but does not call Save().
func GenPrivValidatorFS(filePath string) *PrivValidatorFS {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	return &PrivValidatorFS{
		ID: ValidatorID{privKey.PubKey().Address(), privKey.PubKey()},
		Info: LastSignedInfo{
			LastStep: stepNone,
		},
		Signer:   NewDefaultSigner(privKey),
		filePath: filePath,
	}
}

// LoadPrivValidatorWithSigner loads a PrivValidatorFS with a custom
// signer object. The PrivValidatorFS handles double signing prevention by persisting
// data to the filePath, while the Signer handles the signing.
// If the filePath does not exist, the PrivValidatorFS must be created manually and saved.
func LoadPrivValidatorFSWithSigner(filePath string, signerFunc func(ValidatorID) Signer) *PrivValidatorFS {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := PrivValidatorFS{}
	err = json.Unmarshal(privValJSONBytes, &privVal)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	privVal.filePath = filePath
	privVal.Signer = signerFunc(privVal.ID)
	return &privVal
}

// Address returns the address of the validator.
func (pv *PrivValidatorFS) Address() data.Bytes {
	return pv.ID.Address
}

// PubKey returns the public key of the validator.
func (pv *PrivValidatorFS) PubKey() crypto.PubKey {
	return pv.ID.PubKey
}

// Save persists the PrivValidatorFS to disk.
func (privVal *PrivValidatorFS) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *PrivValidatorFS) save() {
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

// Reset resets all fields in the PrivValidatorFS.Info.
// NOTE: Unsafe!
func (privVal *PrivValidatorFS) Reset() {
	privVal.Info.LastHeight = 0
	privVal.Info.LastRound = 0
	privVal.Info.LastStep = 0
	privVal.Info.LastSignature = crypto.Signature{}
	privVal.Info.LastSignBytes = nil
	privVal.Save()
}

// SignVote signs a canonical representation of the vote, along with the chainID.
func (privVal *PrivValidatorFS) SignVote(chainID string, vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(vote.Height, vote.Round, voteToStep(vote), SignBytes(chainID, vote))
	if err != nil {
		return errors.New(Fmt("Error signing vote: %v", err))
	}
	vote.Signature = signature
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with the chainID.
func (privVal *PrivValidatorFS) SignProposal(chainID string, proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(proposal.Height, proposal.Round, stepPropose, SignBytes(chainID, proposal))
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	proposal.Signature = signature
	return nil
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
func (privVal *PrivValidatorFS) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	var err error
	heartbeat.Signature, err = privVal.Signer.Sign(SignBytes(chainID, heartbeat))
	return err
}

// check if there's a regression. Else sign and write the hrs+signature to disk
func (privVal *PrivValidatorFS) signBytesHRS(height, round int, step int8, signBytes []byte) (crypto.Signature, error) {
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

// String returns a string representation of the PrivValidatorFS.
func (privVal *PrivValidatorFS) String() string {
	info := privVal.Info
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", privVal.Address(), info.LastHeight, info.LastRound, info.LastStep)
}

//-------------------------------------

// ValidatorID contains the identity of the validator.
type ValidatorID struct {
	Address data.Bytes    `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`
}

// LastSignedInfo contains information about the latest
// data signed by a validator to help prevent double signing.
type LastSignedInfo struct {
	LastHeight    int              `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes data.Bytes       `json:"last_signbytes,omitempty"` // so we dont lose signatures
}

// Signer is an interface that defines how to sign messages.
// It is the caller's duty to verify the msg before calling Sign,
// eg. to avoid double signing.
// Currently, the only callers are SignVote, SignProposal, and SignHeartbeat.
type Signer interface {
	Sign(msg []byte) (crypto.Signature, error)
}

// DefaultSigner implements Signer.
// It uses a standard, unencrypted crypto.PrivKey.
type DefaultSigner struct {
	PrivKey crypto.PrivKey `json:"priv_key"`
}

// NewDefaultSigner returns an instance of DefaultSigner.
func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner {
	return &DefaultSigner{
		PrivKey: priv,
	}
}

// Sign implements Signer. It signs the byte slice with a private key.
func (ds *DefaultSigner) Sign(msg []byte) (crypto.Signature, error) {
	return ds.PrivKey.Sign(msg), nil
}

//-------------------------------------

type PrivValidatorsByAddress []*PrivValidatorFS

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address(), pvs[j].Address()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
