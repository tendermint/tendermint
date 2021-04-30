package privval

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"io/ioutil"
	"time"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/libs/tempfile"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// TODO: type ?
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

// A vote is either stepPrevote or stepPrecommit.
func voteToStep(vote *tmproto.Vote) int8 {
	switch vote.Type {
	case tmproto.PrevoteType:
		return stepPrevote
	case tmproto.PrecommitType:
		return stepPrecommit
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

//-------------------------------------------------------------------------------

// FilePVKey stores the immutable part of PrivValidator.
type FilePVKey struct {
	Address            types.Address    `json:"address"`
	PubKey             crypto.PubKey    `json:"pub_key"`
	PrivKey            crypto.PrivKey   `json:"priv_key"`
	NextPrivKeys       []crypto.PrivKey `json:"next_priv_key,omitempty"`
	NextPrivKeyHeights []int64          `json:"next_priv_key_height,omitempty"`
	ProTxHash          crypto.ProTxHash `json:"pro_tx_hash"`

	filePath string
}

// Save persists the FilePVKey to its filePath.
func (pvKey FilePVKey) Save() {
	outFile := pvKey.filePath
	if outFile == "" {
		panic("cannot save PrivValidator key: filePath not set")
	}

	jsonBytes, err := tmjson.MarshalIndent(pvKey, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}

}

//-------------------------------------------------------------------------------

// FilePVLastSignState stores the mutable part of PrivValidator.
type FilePVLastSignState struct {
	Height         int64            `json:"height"`
	Round          int32            `json:"round"`
	Step           int8             `json:"step"`
	BlockSignature []byte           `json:"block_signature,omitempty"`
	BlockSignBytes tmbytes.HexBytes `json:"block_sign_bytes,omitempty"`
	StateSignature []byte           `json:"state_signature,omitempty"`
	StateSignBytes tmbytes.HexBytes `json:"state_sign_bytes,omitempty"`

	filePath string
}

// CheckHRS checks the given height, round, step (HRS) against that of the
// FilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *FilePVLastSignState) CheckHRS(height int64, round int32, step int8) (bool, error) {

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
				if lss.BlockSignBytes != nil {
					if lss.BlockSignature == nil {
						panic("pv: BlockID Signature is nil but BlockSignBytes is not!")
					}
					return true, nil
				}
				if lss.StateSignBytes != nil {
					if lss.StateSignature == nil {
						panic("pv: StateID Signature is nil but StateSignBytes is not!")
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
func (lss *FilePVLastSignState) Save() {
	outFile := lss.filePath
	if outFile == "" {
		panic("cannot save FilePVLastSignState: filePath not set")
	}
	jsonBytes, err := tmjson.MarshalIndent(lss, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
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

// NewFilePV generates a new validator from the given key and paths.
func NewFilePV(privKey crypto.PrivKey, proTxHash []byte, nextPrivKeys []crypto.PrivKey, nextPrivHeights []int64,
	keyFilePath, stateFilePath string) *FilePV {
	if len(proTxHash) != crypto.ProTxHashSize {
		panic("error setting incorrect proTxHash size in NewFilePV")
	}

	return &FilePV{
		Key: FilePVKey{
			Address:            privKey.PubKey().Address(),
			PubKey:             privKey.PubKey(),
			PrivKey:            privKey,
			NextPrivKeys:       nextPrivKeys,
			NextPrivKeyHeights: nextPrivHeights,
			ProTxHash:          proTxHash,
			filePath:           keyFilePath,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenFilePV(keyFilePath, stateFilePath string) *FilePV {
	return NewFilePV(bls12381.GenPrivKey(), crypto.RandProTxHash(), nil, nil,
		keyFilePath, stateFilePath)
}

// LoadFilePV loads a FilePV from the filePaths.  The FilePV handles double
// signing prevention by persisting data to the stateFilePath.  If either file path
// does not exist, the program will exit.
func LoadFilePV(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, true)
}

// LoadFilePVEmptyState loads a FilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
func LoadFilePVEmptyState(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, false)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadFilePV(keyFilePath, stateFilePath string, loadState bool) *FilePV {
	keyJSONBytes, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		tmos.Exit(err.Error())
	}
	pvKey := FilePVKey{}
	err = tmjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		tmos.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}
	// verify proTxHash is 32 bytes if it exists
	if pvKey.ProTxHash != nil && len(pvKey.ProTxHash) != crypto.ProTxHashSize {
		tmos.Exit(fmt.Sprintf("loadFilePV proTxHash must be 32 bytes in key file path %s", keyFilePath))
	}

	// overwrite pubkey and address for convenience
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = pvKey.PubKey.Address()
	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := ioutil.ReadFile(stateFilePath)
		if err != nil {
			tmos.Exit(err.Error())
		}
		err = tmjson.Unmarshal(stateJSONBytes, &pvState)
		if err != nil {
			tmos.Exit(fmt.Sprintf("Error reading PrivValidator state from %v: %v\n", stateFilePath, err))
		}
	}

	pvState.filePath = stateFilePath

	return &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}
}

// LoadOrGenFilePV loads a FilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenFilePV(keyFilePath, stateFilePath string) *FilePV {
	var pv *FilePV
	if tmos.FileExists(keyFilePath) {
		pv = LoadFilePV(keyFilePath, stateFilePath)
	} else {
		pv = GenFilePV(keyFilePath, stateFilePath)
		pv.Save()
	}
	return pv
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() types.Address {
	return pv.Key.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	return pv.Key.PubKey, nil
}

func (pv *FilePV) ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *types.Validator {
	var pubKey crypto.PubKey
	if pv.Key.NextPrivKeys != nil && len(pv.Key.NextPrivKeys) > 0 && height >= pv.Key.NextPrivKeyHeights[0] {
		for i, nextPrivKeyHeight := range pv.Key.NextPrivKeyHeights {
			if height >= nextPrivKeyHeight {
				pubKey = pv.Key.NextPrivKeys[i].PubKey()
			}
		}
	} else {
		pubKey, _ = pv.GetPubKey(quorumHash)
	}
	if len(pv.Key.ProTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   pv.Key.ProTxHash,
	}
}

// GetProTxHash returns the pro tx hash of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetProTxHash() (crypto.ProTxHash, error) {
	if len(pv.Key.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("file proTxHash is invalid size")
	}
	return pv.Key.ProTxHash, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	if err := pv.signVote(chainID, quorumType, quorumHash, vote); err != nil {
		return fmt.Errorf("error signing vote: %v", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	if err := pv.signProposal(chainID, quorumType, quorumHash, proposal); err != nil {
		return fmt.Errorf("error signing proposal: %v", err)
	}
	return nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.Key.Save()
	pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var blockSig []byte
	var stateSig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.BlockSignature = blockSig
	pv.LastSignState.StateSignature = stateSig
	pv.LastSignState.BlockSignBytes = nil
	pv.LastSignState.StateSignBytes = nil
	pv.Save()
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

func (pv *FilePV) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	pv.Key.NextPrivKeys = append(pv.Key.NextPrivKeys, privateKey)
	pv.Key.NextPrivKeyHeights = append(pv.Key.NextPrivKeyHeights, height)
	return nil
}

func (pv *FilePV) updateKeyIfNeeded(height int64) {
	if pv.Key.NextPrivKeys != nil && len(pv.Key.NextPrivKeys) > 0 && pv.Key.NextPrivKeyHeights != nil &&
		len(pv.Key.NextPrivKeyHeights) > 0 && height >= pv.Key.NextPrivKeyHeights[0] {
		// fmt.Printf("privval file node %X at height %d updating key %X with new key %X\n", pv.Key.ProTxHash, height,
		//  pv.Key.PrivKey.PubKey().Bytes(), pv.Key.NextPrivKeys[0].PubKey().Bytes())
		pv.Key.PrivKey = pv.Key.NextPrivKeys[0]
		if len(pv.Key.NextPrivKeys) > 1 {
			pv.Key.NextPrivKeys = pv.Key.NextPrivKeys[1:]
			pv.Key.NextPrivKeyHeights = pv.Key.NextPrivKeyHeights[1:]
		} else {
			pv.Key.NextPrivKeys = nil
			pv.Key.NextPrivKeyHeights = nil
		}
	}
	// else {
	// fmt.Printf("privval file node %X at height %d did not update key %X with next keys %v\n", pv.Key.ProTxHash,
	//  height, pv.Key.PrivKey.PubKey().Bytes(), pv.Key.NextPrivKeyHeights)
	// }
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	pv.updateKeyIfNeeded(vote.Height)
	height, round, step := vote.Height, vote.Round, voteToStep(vote)

	lss := pv.LastSignState

	// The vote should not have a state ID set if the block ID is set to nil

	if vote.BlockID.Hash == nil && vote.StateID.LastAppHash != nil {
		return fmt.Errorf("error : vote should not have a state ID set if the"+
			" block ID for the round (%d/%d) is not set", vote.Height, vote.Round)
	}

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	blockSignId := types.VoteBlockSignId(chainID, vote, quorumType, quorumHash)

	stateSignId := types.VoteStateSignId(chainID, vote, quorumType, quorumHash)

	blockSignBytes := types.VoteBlockSignBytes(chainID, vote)

	stateSignBytes := types.VoteStateSignBytes(chainID, vote)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {

		if bytes.Equal(blockSignBytes, lss.BlockSignBytes) && bytes.Equal(stateSignBytes, lss.StateSignBytes) {
			vote.BlockSignature = lss.BlockSignature
			vote.StateSignature = lss.StateSignature
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	sigBlock, err := pv.Key.PrivKey.SignDigest(blockSignId)
	if err != nil {
		return err
	}

	var sigState []byte
	if vote.BlockID.Hash != nil {
		sigState, err = pv.Key.PrivKey.SignDigest(stateSignId)
		if err != nil {
			return err
		}
	}

	//  if vote.BlockID.Hash == nil {
	//	  fmt.Printf("***********we are signing NIL (%d/%d) %X signed (file) for vote %v blockSignBytes %X\n",
	//	    vote.Height, vote.Round, sigBlock, vote, blockSignBytes)
	//  } else {
	//	  fmt.Printf("==block signature (%d/%d) %X signed (file) for vote %v\n", vote.Height, vote.Round,
	//	   sigBlock, vote)
	//  }

	pv.saveSigned(height, round, step, blockSignBytes, sigBlock, stateSignBytes, sigState)

	vote.BlockSignature = sigBlock
	vote.StateSignature = sigState

	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	pv.updateKeyIfNeeded(proposal.Height)
	height, round, step := proposal.Height, proposal.Round, stepPropose

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	blockSignId := types.ProposalBlockSignId(chainID, proposal, quorumType, quorumHash)

	blockSignBytes := types.ProposalBlockSignBytes(chainID, proposal)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(blockSignBytes, lss.BlockSignBytes) {
			proposal.Signature = lss.BlockSignBytes
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lss.BlockSignBytes, blockSignBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = lss.BlockSignBytes
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	// It passed the checks. SignDigest the proposal
	blockSig, err := pv.Key.PrivKey.SignDigest(blockSignId)
	if err != nil {
		return err
	}
	// fmt.Printf("file proposer %X \nsigning proposal at height %d \nwith key %X \nproposalSignId %X\n signature %X\n", pv.Key.ProTxHash,
	//  proposal.Height, pv.Key.PrivKey.PubKey().Bytes(), blockSignId, blockSig)

	pv.saveSigned(height, round, step, blockSignBytes, blockSig, nil, nil)
	proposal.Signature = blockSig
	return nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height int64, round int32, step int8,
	blockSignBytes []byte, blockSig []byte, stateSignBytes []byte, stateSig []byte) {

	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.BlockSignature = blockSig
	pv.LastSignState.BlockSignBytes = blockSignBytes
	pv.LastSignState.StateSignature = stateSig
	pv.LastSignState.StateSignBytes = stateSignBytes
	pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal tmproto.CanonicalProposal
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime := lastProposal.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastProposal.Timestamp = now
	newProposal.Timestamp = now

	return lastTime, proto.Equal(&newProposal, &lastProposal)
}
