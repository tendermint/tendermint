package privval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/internal/libs/tempfile"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// TODO: type ?
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

// A vote is either stepPrevote or stepPrecommit.
func voteToStep(vote *tmproto.Vote) (int8, error) {
	switch vote.Type {
	case tmproto.PrevoteType:
		return stepPrevote, nil
	case tmproto.PrecommitType:
		return stepPrecommit, nil
	default:
		return 0, fmt.Errorf("unknown vote type: %v", vote.Type)
	}
}

//-------------------------------------------------------------------------------

// FilePVKey stores the immutable part of PrivValidator.
type FilePVKey struct {
	Address types.Address
	PubKey  crypto.PubKey
	PrivKey crypto.PrivKey

	filePath string
}

type filePVKeyJSON struct {
	Address types.Address   `json:"address"`
	PubKey  json.RawMessage `json:"pub_key"`
	PrivKey json.RawMessage `json:"priv_key"`
}

func (pvKey FilePVKey) MarshalJSON() ([]byte, error) {
	pubk, err := jsontypes.Marshal(pvKey.PubKey)
	if err != nil {
		return nil, err
	}
	privk, err := jsontypes.Marshal(pvKey.PrivKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(filePVKeyJSON{
		Address: pvKey.Address, PubKey: pubk, PrivKey: privk,
	})
}

func (pvKey *FilePVKey) UnmarshalJSON(data []byte) error {
	var key filePVKeyJSON
	if err := json.Unmarshal(data, &key); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(key.PubKey, &pvKey.PubKey); err != nil {
		return fmt.Errorf("decoding PubKey: %w", err)
	}
	if err := jsontypes.Unmarshal(key.PrivKey, &pvKey.PrivKey); err != nil {
		return fmt.Errorf("decoding PrivKey: %w", err)
	}
	pvKey.Address = key.Address
	return nil
}

// Save persists the FilePVKey to its filePath.
func (pvKey FilePVKey) Save() error {
	outFile := pvKey.filePath
	if outFile == "" {
		return errors.New("cannot save PrivValidator key: filePath not set")
	}

	data, err := json.MarshalIndent(pvKey, "", "  ")
	if err != nil {
		return err
	}
	return tempfile.WriteFileAtomic(outFile, data, 0600)
}

//-------------------------------------------------------------------------------

// FilePVLastSignState stores the mutable part of PrivValidator.
type FilePVLastSignState struct {
	Height    int64            `json:"height,string"`
	Round     int32            `json:"round"`
	Step      int8             `json:"step"`
	Signature []byte           `json:"signature,omitempty"`
	SignBytes tmbytes.HexBytes `json:"signbytes,omitempty"`

	filePath string
}

func (lss *FilePVLastSignState) reset() {
	lss.Height = 0
	lss.Round = 0
	lss.Step = 0
	lss.Signature = nil
	lss.SignBytes = nil
}

// checkHRS checks the given height, round, step (HRS) against that of the
// FilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *FilePVLastSignState) checkHRS(height int64, round int32, step int8) (bool, error) {

	if lss.Height > height {
		return false, fmt.Errorf("height regression. Got %v, last height %v", height, lss.Height)
	}

	if lss.Height == height {
		if lss.Round > round {
			return false, fmt.Errorf("round regression at height %v. Got %v, last round %v", height, round, lss.Round)
		}

		if lss.Round == round {
			if lss.Step > step {
				return false, fmt.Errorf(
					"step regression at height %v round %v. Got %v, last step %v",
					height,
					round,
					step,
					lss.Step,
				)
			} else if lss.Step == step {
				if lss.SignBytes != nil {
					if lss.Signature == nil {
						panic("pv: Signature is nil but SignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("no SignBytes found")
			}
		}
	}
	return false, nil
}

// Save persists the FilePvLastSignState to its filePath.
func (lss *FilePVLastSignState) Save() error {
	outFile := lss.filePath
	if outFile == "" {
		return errors.New("cannot save FilePVLastSignState: filePath not set")
	}
	jsonBytes, err := json.MarshalIndent(lss, "", "  ")
	if err != nil {
		return err
	}
	return tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
}

//-------------------------------------------------------------------------------

// FilePV implements PrivValidator using data persisted to disk
// to prevent double signing.
// NOTE: the directories containing pv.Key.filePath and pv.LastSignState.filePath must already exist.
// It includes the LastSignature and LastSignBytes so we don't lose the signature
// if the process crashes after signing but before the resulting consensus message is processed.
type FilePV struct {
	Key           FilePVKey
	LastSignState FilePVLastSignState
}

var _ types.PrivValidator = (*FilePV)(nil)

// NewFilePV generates a new validator from the given key and paths.
func NewFilePV(privKey crypto.PrivKey, keyFilePath, stateFilePath string) *FilePV {
	return &FilePV{
		Key: FilePVKey{
			Address:  privKey.PubKey().Address(),
			PubKey:   privKey.PubKey(),
			PrivKey:  privKey,
			filePath: keyFilePath,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenFilePV(keyFilePath, stateFilePath, keyType string) (*FilePV, error) {
	switch keyType {
	case types.ABCIPubKeyTypeSecp256k1:
		return NewFilePV(secp256k1.GenPrivKey(), keyFilePath, stateFilePath), nil
	case "", types.ABCIPubKeyTypeEd25519:
		return NewFilePV(ed25519.GenPrivKey(), keyFilePath, stateFilePath), nil
	default:
		return nil, fmt.Errorf("key type: %s is not supported", keyType)
	}
}

// LoadFilePV loads a FilePV from the filePaths.  The FilePV handles double
// signing prevention by persisting data to the stateFilePath.  If either file path
// does not exist, the program will exit.
func LoadFilePV(keyFilePath, stateFilePath string) (*FilePV, error) {
	return loadFilePV(keyFilePath, stateFilePath, true)
}

// LoadFilePVEmptyState loads a FilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
func LoadFilePVEmptyState(keyFilePath, stateFilePath string) (*FilePV, error) {
	return loadFilePV(keyFilePath, stateFilePath, false)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadFilePV(keyFilePath, stateFilePath string, loadState bool) (*FilePV, error) {
	keyJSONBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	pvKey := FilePVKey{}
	err = json.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		return nil, fmt.Errorf("error reading PrivValidator key from %v: %w", keyFilePath, err)
	}

	// overwrite pubkey and address for convenience
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = pvKey.PubKey.Address()
	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := os.ReadFile(stateFilePath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(stateJSONBytes, &pvState)
		if err != nil {
			return nil, fmt.Errorf("error reading PrivValidator state from %v: %w", stateFilePath, err)
		}
	}

	pvState.filePath = stateFilePath

	return &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}, nil
}

// LoadOrGenFilePV loads a FilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenFilePV(keyFilePath, stateFilePath string) (*FilePV, error) {
	if tmos.FileExists(keyFilePath) {
		pv, err := LoadFilePV(keyFilePath, stateFilePath)
		if err != nil {
			return nil, err
		}
		return pv, nil
	}
	pv, err := GenFilePV(keyFilePath, stateFilePath, "")
	if err != nil {
		return nil, err
	}

	if err := pv.Save(); err != nil {
		return nil, err
	}

	return pv, nil
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() types.Address {
	return pv.Key.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey(ctx context.Context) (crypto.PubKey, error) {
	return pv.Key.PubKey, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(ctx context.Context, chainID string, vote *tmproto.Vote) error {
	if err := pv.signVote(chainID, vote); err != nil {
		return fmt.Errorf("error signing vote: %w", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(ctx context.Context, chainID string, proposal *tmproto.Proposal) error {
	if err := pv.signProposal(chainID, proposal); err != nil {
		return fmt.Errorf("error signing proposal: %w", err)
	}
	return nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() error {
	if err := pv.Key.Save(); err != nil {
		return err
	}
	return pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() error {
	pv.LastSignState.reset()
	return pv.Save()
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.GetAddress(),
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, vote *tmproto.Vote) error {
	step, err := voteToStep(vote)
	if err != nil {
		return err
	}

	height := vote.Height
	round := vote.Round
	lss := pv.LastSignState

	sameHRS, err := lss.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := types.VoteSignBytes(chainID, vote)

	// Vote extensions are non-deterministic, so it is possible that an
	// application may have created a different extension. We therefore always
	// re-sign the vote extensions of precommits. For prevotes and nil
	// precommits, the extension signature will always be empty.
	var extSig []byte
	if vote.Type == tmproto.PrecommitType && !types.ProtoBlockIDIsNil(&vote.BlockID) {
		extSignBytes := types.VoteExtensionSignBytes(chainID, vote)
		extSig, err = pv.Key.PrivKey.Sign(extSignBytes)
		if err != nil {
			return err
		}
	} else if len(vote.Extension) > 0 {
		return errors.New("unexpected vote extension - extensions are only allowed in non-nil precommits")
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lss.SignBytes) {
			vote.Signature = lss.Signature
		} else {
			// Compares the canonicalized votes (i.e. without vote extensions
			// or vote extension signatures).
			timestamp, ok, err := checkVotesOnlyDifferByTimestamp(lss.SignBytes, signBytes)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("conflicting data")
			}

			vote.Timestamp = timestamp
			vote.Signature = lss.Signature
		}

		vote.ExtensionSignature = extSig

		return nil
	}

	// It passed the checks. Sign the vote
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	if err := pv.saveSigned(height, round, step, signBytes, sig); err != nil {
		return err
	}
	vote.Signature = sig
	vote.ExtensionSignature = extSig

	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
func (pv *FilePV) signProposal(chainID string, proposal *tmproto.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose

	lss := pv.LastSignState

	sameHRS, err := lss.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := types.ProposalSignBytes(chainID, proposal)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	if sameHRS {
		if !bytes.Equal(signBytes, lss.SignBytes) {
			return errors.New("conflicting data")
		}
		proposal.Signature = lss.Signature
		return nil
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	if err := pv.saveSigned(height, round, step, signBytes, sig); err != nil {
		return err
	}
	proposal.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height int64, round int32, step int8, signBytes []byte, sig []byte) error {
	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = signBytes
	return pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// Returns the timestamp from the lastSignBytes.
// Returns true if the only difference in the votes is their timestamp.
// Performs these checks on the canonical votes (excluding the vote extension
// and vote extension signatures).
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool, error) {
	var lastVote, newVote tmproto.CanonicalVote
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastVote); err != nil {
		return time.Time{}, false, fmt.Errorf("LastSignBytes cannot be unmarshalled into vote: %w", err)
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newVote); err != nil {
		return time.Time{}, false, fmt.Errorf("signBytes cannot be unmarshalled into vote: %w", err)
	}

	lastTime := lastVote.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastVote.Timestamp = now
	newVote.Timestamp = now

	return lastTime, proto.Equal(&newVote, &lastVote), nil
}
