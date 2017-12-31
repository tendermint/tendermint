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

// Address returns the address of the validator.
// Implements PrivValidator.
func (pv *PrivValidatorFS) Address() data.Bytes {
	return pv.ID.Address
}

// PubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *PrivValidatorFS) PubKey() crypto.PubKey {
	return pv.ID.PubKey
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

// LoadPrivValidatorFS loads a PrivValidatorFS from the filePath.
func LoadPrivValidatorFS(filePath string) *PrivValidatorFS {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	privVal := PrivValidatorFS{}
	err = json.Unmarshal(privValJSONBytes, &privVal)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	privVal.filePath = filePath
	return &privVal
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

// UnmarshalJSON unmarshals the given jsonString
// into a PrivValidatorFS using a DefaultSigner.
func (pv *PrivValidatorFS) UnmarshalJSON(jsonString []byte) error {
	idAndInfo := &struct {
		ID   ValidatorID    `json:"id"`
		Info LastSignedInfo `json:"info"`
	}{}
	if err := json.Unmarshal(jsonString, idAndInfo); err != nil {
		return err
	}

	signer := &struct {
		Signer *DefaultSigner `json:"signer"`
	}{}
	if err := json.Unmarshal(jsonString, signer); err != nil {
		return err
	}
	fmt.Println("STRING", string(jsonString))
	fmt.Println("SIGNER", signer)

	pv.ID = idAndInfo.ID
	pv.Info = idAndInfo.Info
	pv.Signer = signer.Signer
	return nil
}

// Reset resets all fields in the PrivValidatorFS.
// NOTE: Unsafe!
func (privVal *PrivValidatorFS) Reset() {
	privVal.Info.Reset()
	privVal.Save()
}

func (info *LastSignedInfo) Reset() {
	info.LastHeight = 0
	info.LastRound = 0
	info.LastStep = 0
	info.LastSignature = crypto.Signature{}
	info.LastSignBytes = nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (privVal *PrivValidatorFS) SignVote(chainID string, vote *Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.Info.SignBytesHRS(privVal.Signer, vote.Height, vote.Round, voteToStep(vote),
		SignBytes(chainID, vote), checkVotesOnlyDifferByTimestamp)
	if err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	privVal.save()
	vote.Signature = signature
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (privVal *PrivValidatorFS) SignProposal(chainID string, proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	signature, err := privVal.Info.SignBytesHRS(privVal.Signer, proposal.Height, proposal.Round, stepPropose,
		SignBytes(chainID, proposal), checkProposalsOnlyDifferByTimestamp)
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	privVal.save()
	proposal.Signature = signature
	return nil
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (privVal *PrivValidatorFS) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	var err error
	heartbeat.Signature, err = privVal.Signer.Sign(SignBytes(chainID, heartbeat))
	return err
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

//-------------------------------------

// LastSignedInfo contains information about the latest
// data signed by a validator to help prevent double signing.
type LastSignedInfo struct {
	LastHeight    int64            `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes data.Bytes       `json:"last_signbytes,omitempty"` // so we dont lose signatures
}

// returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (info *LastSignedInfo) checkHRS(height int64, round int, step int8) (bool, error) {
	if info.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if info.LastHeight == height {
		if info.LastRound > round {
			return false, errors.New("Round regression")
		}

		if info.LastRound == round {
			if info.LastStep > step {
				return false, errors.New("Step regression")
			} else if info.LastStep == step {
				if info.LastSignBytes != nil {
					if info.LastSignature.Empty() {
						panic("info: LastSignature is nil but LastSignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("No LastSignature found")
			}
		}
	}
	return false, nil
}

// SignBytesHRS signs the given signBytes with the signer if the height/round/step (HRS) are
// greater than the latest state of the LastSignedInfo. If the HRS are equal and the only thing changed is the timestamp,
// it returns the privValidator.LastSignature. Else it returns an error.
func (info *LastSignedInfo) SignBytesHRS(signer Signer, height int64, round int, step int8,
	signBytes []byte, checkFn checkOnlyDifferByTimestamp) (crypto.Signature, error) {

	sig := crypto.Signature{}

	sameHRS, err := info.checkHRS(height, round, step)
	if err != nil {
		return sig, err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS
	if sameHRS {
		// if they're the same or only differ by timestamp,
		// return the LastSignature. Otherwise, error
		if bytes.Equal(signBytes, info.LastSignBytes) ||
			checkFn(info.LastSignBytes, signBytes) {
			return info.LastSignature, nil
		}
		return sig, fmt.Errorf("Conflicting data")
	}

	// Sign
	sig, err = signer.Sign(signBytes)
	if err != nil {
		return sig, err
	}
	info.setSigned(height, round, step, signBytes, sig)
	return sig, nil
}

// Set height/round/step and signature on the info
func (info *LastSignedInfo) setSigned(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	info.LastHeight = height
	info.LastRound = round
	info.LastStep = step
	info.LastSignature = sig
	info.LastSignBytes = signBytes
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
