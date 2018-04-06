package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

// TODO: type ?
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
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

//--------------------------------------------------------------
// PrivValidator is being upgraded! See types/priv_validator/

// ValidatorID contains the identity of the validator.
type ValidatorID struct {
	Address cmn.HexBytes  `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`
}

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator2 interface {
	Address() (Address, error) // redundant since .PubKey().Address()
	PubKey() (crypto.PubKey, error)

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

type TestSigner interface {
	Address() cmn.HexBytes
	PubKey() crypto.PubKey
	Sign([]byte) (crypto.Signature, error)
}

func GenSigner() TestSigner {
	return &DefaultTestSigner{
		crypto.GenPrivKeyEd25519().Wrap(),
	}
}

type DefaultTestSigner struct {
	crypto.PrivKey
}

func (ds *DefaultTestSigner) Address() cmn.HexBytes {
	return ds.PubKey().Address()
}

func (ds *DefaultTestSigner) PubKey() crypto.PubKey {
	return ds.PrivKey.PubKey()
}

func (ds *DefaultTestSigner) Sign(msg []byte) (crypto.Signature, error) {
	return ds.PrivKey.Sign(msg), nil
}

//--------------------------------------------------------------
// TODO: Deprecate!

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator interface {
	GetAddress() Address // redundant since .PubKey().Address()
	GetPubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

// PrivValidatorFS implements PrivValidator using data persisted to disk
// to prevent double signing. The Signer itself can be mutated to use
// something besides the default, for instance a hardware signer.
// NOTE: the directory containing the privVal.filePath must already exist.
type PrivValidatorFS struct {
	Address       Address          `json:"address"`
	PubKey        crypto.PubKey    `json:"pub_key"`
	LastHeight    int64            `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes cmn.HexBytes     `json:"last_signbytes,omitempty"` // so we dont lose signatures

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
func (pv *PrivValidatorFS) GetAddress() Address {
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
	if cmn.FileExists(filePath) {
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
func (pv *PrivValidatorFS) Save() {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	pv.save()
}

func (pv *PrivValidatorFS) save() {
	outFile := pv.filePath
	if outFile == "" {
		panic("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(pv)
	if err != nil {
		panic(err)
	}
	err = cmn.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

// Reset resets all fields in the PrivValidatorFS.
// NOTE: Unsafe!
func (pv *PrivValidatorFS) Reset() {
	var sig crypto.Signature
	pv.LastHeight = 0
	pv.LastRound = 0
	pv.LastStep = 0
	pv.LastSignature = sig
	pv.LastSignBytes = nil
	pv.Save()
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *PrivValidatorFS) SignVote(chainID string, vote *Vote) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signVote(chainID, vote); err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *PrivValidatorFS) SignProposal(chainID string, proposal *Proposal) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signProposal(chainID, proposal); err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	return nil
}

// returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (pv *PrivValidatorFS) checkHRS(height int64, round int, step int8) (bool, error) {
	if pv.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if pv.LastHeight == height {
		if pv.LastRound > round {
			return false, errors.New("Round regression")
		}

		if pv.LastRound == round {
			if pv.LastStep > step {
				return false, errors.New("Step regression")
			} else if pv.LastStep == step {
				if pv.LastSignBytes != nil {
					if pv.LastSignature.Empty() {
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

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *PrivValidatorFS) signVote(chainID string, vote *Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)
	signBytes := vote.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			vote.Signature = pv.LastSignature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := pv.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	vote.Signature = sig
	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *PrivValidatorFS) signProposal(chainID string, proposal *Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := proposal.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			proposal.Signature = pv.LastSignature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *PrivValidatorFS) saveSigned(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	pv.LastHeight = height
	pv.LastRound = round
	pv.LastStep = step
	pv.LastSignature = sig
	pv.LastSignBytes = signBytes
	pv.save()
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (pv *PrivValidatorFS) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	var err error
	heartbeat.Signature, err = pv.Sign(heartbeat.SignBytes(chainID))
	return err
}

// String returns a string representation of the PrivValidatorFS.
func (pv *PrivValidatorFS) String() string {
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", pv.GetAddress(), pv.LastHeight, pv.LastRound, pv.LastStep)
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

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote CanonicalJSONOnceVote
	if err := json.Unmarshal(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime, err := time.Parse(TimeFormat, lastVote.Vote.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastVote.Vote.Timestamp = now
	newVote.Vote.Timestamp = now
	lastVoteBytes, _ := json.Marshal(lastVote)
	newVoteBytes, _ := json.Marshal(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal CanonicalJSONOnceProposal
	if err := json.Unmarshal(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime, err := time.Parse(TimeFormat, lastProposal.Proposal.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastProposal.Proposal.Timestamp = now
	newProposal.Proposal.Timestamp = now
	lastProposalBytes, _ := json.Marshal(lastProposal)
	newProposalBytes, _ := json.Marshal(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes)
}
