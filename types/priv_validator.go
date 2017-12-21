package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
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
		cmn.PanicSanity("Unknown vote type")
		return 0
	}
}

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator interface {
	GetAddress() data.Bytes // redundant since .PubKey().Address()
	GetPubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

// PrivValidatorFS implements PrivValidator using data persisted to disk
// to prevent double signing. The Signer itself can be mutated to use
// something besides the default, for instance a hardware signer.
type PrivValidatorFS struct {
	Address       data.Bytes       `json:"address"`
	PubKey        crypto.PubKey    `json:"pub_key"`
	LastHeight    int64            `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes data.Bytes       `json:"last_signbytes,omitempty"` // so we dont lose signatures

	// PrivKey should be empty if a Signer other than the default is being used.
	PrivKey crypto.PrivKey `json:"priv_key"`
	Signer  `json:"-"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
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

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *PrivValidatorFS) GetAddress() data.Bytes {
	return pv.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *PrivValidatorFS) GetPubKey() crypto.PubKey {
	return pv.PubKey
}

// GenPrivValidatorFS generates a new validator with randomly generated private key
// and sets the filePath, but does not call Save().
func GenPrivValidatorFS(filePath string) *PrivValidatorFS {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	return &PrivValidatorFS{
		Address:  privKey.PubKey().Address(),
		PubKey:   privKey.PubKey(),
		PrivKey:  privKey,
		LastStep: stepNone,
		Signer:   NewDefaultSigner(privKey),
		filePath: filePath,
	}
}

// LoadPrivValidatorFS loads a PrivValidatorFS from the filePath.
func LoadPrivValidatorFS(filePath string) *PrivValidatorFS {
	return LoadPrivValidatorFSWithSigner(filePath, func(privVal PrivValidator) Signer {
		return NewDefaultSigner(privVal.(*PrivValidatorFS).PrivKey)
	})
}

// LoadOrGenPrivValidatorFS loads a PrivValidatorFS from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenPrivValidatorFS(filePath string) *PrivValidatorFS {
	var privVal *PrivValidatorFS
	if _, err := os.Stat(filePath); err == nil {
		privVal = LoadPrivValidatorFS(filePath)
	} else {
		privVal = GenPrivValidatorFS(filePath)
		privVal.Save()
	}
	return privVal
}

// LoadPrivValidatorWithSigner loads a PrivValidatorFS with a custom
// signer object. The PrivValidatorFS handles double signing prevention by persisting
// data to the filePath, while the Signer handles the signing.
// If the filePath does not exist, the PrivValidatorFS must be created manually and saved.
func LoadPrivValidatorFSWithSigner(filePath string, signerFunc func(PrivValidator) Signer) *PrivValidatorFS {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	privVal := &PrivValidatorFS{}
	err = json.Unmarshal(privValJSONBytes, &privVal)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	privVal.filePath = filePath
	privVal.Signer = signerFunc(privVal)
	return privVal
}

// Save persists the PrivValidatorFS to disk.
func (privVal *PrivValidatorFS) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *PrivValidatorFS) save() {
	if privVal.filePath == "" {
		cmn.PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(privVal)
	if err != nil {
		// `@; BOOM!!!
		cmn.PanicCrisis(err)
	}
	err = cmn.WriteFileAtomic(privVal.filePath, jsonBytes, 0600)
	if err != nil {
		// `@; BOOM!!!
		cmn.PanicCrisis(err)
	}
}

// Reset resets all fields in the PrivValidatorFS.
// NOTE: Unsafe!
func (privVal *PrivValidatorFS) Reset() {
	privVal.LastHeight = 0
	privVal.LastRound = 0
	privVal.LastStep = 0
	privVal.LastSignature = crypto.Signature{}
	privVal.LastSignBytes = nil
	privVal.Save()
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (privVal *PrivValidatorFS) SignVote(chainID string, vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(vote.Height, vote.Round, voteToStep(vote),
		SignBytes(chainID, vote), checkVotesOnlyDifferByTimestamp)
	if err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	vote.Signature = signature
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (privVal *PrivValidatorFS) SignProposal(chainID string, proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.signBytesHRS(proposal.Height, proposal.Round, stepPropose,
		SignBytes(chainID, proposal), checkProposalsOnlyDifferByTimestamp)
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	proposal.Signature = signature
	return nil
}

// returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (privVal *PrivValidatorFS) checkHRS(height int64, round int, step int8) (bool, error) {
	if privVal.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if privVal.LastHeight == height {
		if privVal.LastRound > round {
			return false, errors.New("Round regression")
		}

		if privVal.LastRound == round {
			if privVal.LastStep > step {
				return false, errors.New("Step regression")
			} else if privVal.LastStep == step {
				if privVal.LastSignBytes != nil {
					if privVal.LastSignature.Empty() {
						panic("privVal: LastSignature is nil but LastSignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("No LastSignature found")
			}
		}
	}
	return false, nil
}

// signBytesHRS signs the given signBytes if the height/round/step (HRS) are
// greater than the latest state. If the HRS are equal and the only thing changed is the timestamp,
// it returns the privValidator.LastSignature. Else it returns an error.
func (privVal *PrivValidatorFS) signBytesHRS(height int64, round int, step int8,
	signBytes []byte, checkFn checkOnlyDifferByTimestamp) (crypto.Signature, error) {
	sig := crypto.Signature{}

	sameHRS, err := privVal.checkHRS(height, round, step)
	if err != nil {
		return sig, err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS
	if sameHRS {
		// if they're the same or only differ by timestamp,
		// return the LastSignature. Otherwise, error
		if bytes.Equal(signBytes, privVal.LastSignBytes) ||
			checkFn(privVal.LastSignBytes, signBytes) {
			return privVal.LastSignature, nil
		}
		return sig, fmt.Errorf("Conflicting data")
	}

	sig, err = privVal.Sign(signBytes)
	if err != nil {
		return sig, err
	}
	privVal.saveSigned(height, round, step, signBytes, sig)
	return sig, nil
}

// Persist height/round/step and signature
func (privVal *PrivValidatorFS) saveSigned(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	privVal.LastHeight = height
	privVal.LastRound = round
	privVal.LastStep = step
	privVal.LastSignature = sig
	privVal.LastSignBytes = signBytes
	privVal.save()
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (privVal *PrivValidatorFS) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	var err error
	heartbeat.Signature, err = privVal.Sign(SignBytes(chainID, heartbeat))
	return err
}

// String returns a string representation of the PrivValidatorFS.
func (privVal *PrivValidatorFS) String() string {
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", privVal.GetAddress(), privVal.LastHeight, privVal.LastRound, privVal.LastStep)
}

//-------------------------------------

type PrivValidatorsByAddress []*PrivValidatorFS

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].GetAddress(), pvs[j].GetAddress()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}

//-------------------------------------

type checkOnlyDifferByTimestamp func([]byte, []byte) bool

// returns true if the only difference in the votes is their timestamp
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) bool {
	var lastVote, newVote CanonicalJSONOnceVote
	if err := json.Unmarshal(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastVote.Vote.Timestamp = now
	newVote.Vote.Timestamp = now
	lastVoteBytes, _ := json.Marshal(lastVote)
	newVoteBytes, _ := json.Marshal(newVote)

	return bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) bool {
	var lastProposal, newProposal CanonicalJSONOnceProposal
	if err := json.Unmarshal(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastProposal.Proposal.Timestamp = now
	newProposal.Proposal.Timestamp = now
	lastProposalBytes, _ := json.Marshal(lastProposal)
	newProposalBytes, _ := json.Marshal(newProposal)

	return bytes.Equal(newProposalBytes, lastProposalBytes)
}
