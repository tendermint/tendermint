package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"strconv"

	tmsync "github.com/tendermint/tendermint/libs/sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes and proposals, and never double signs.
type PrivValidator interface {
	GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error)
	UpdatePrivateKey(privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, height int64) error

	GetProTxHash() (crypto.ProTxHash, error)
	GetFirstQuorumHash() (crypto.QuorumHash, error)
	GetThresholdPublicKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error)
	GetHeight(quorumHash crypto.QuorumHash) (int64, error)

	SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error
	SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) ([]byte, error)

	ExtractIntoValidator(quorumHash crypto.QuorumHash) *Validator
}

type PrivValidatorsByProTxHash []PrivValidator

func (pvs PrivValidatorsByProTxHash) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByProTxHash) Less(i, j int) bool {
	pvi, err := pvs[i].GetProTxHash()
	if err != nil {
		panic(err)
	}
	pvj, err := pvs[j].GetProTxHash()
	if err != nil {
		panic(err)
	}

	return bytes.Compare(pvi, pvj) == -1
}

func (pvs PrivValidatorsByProTxHash) Swap(i, j int) {
	pvs[i], pvs[j] = pvs[j], pvs[i]
}

//----------------------------------------
// MockPV

// MockPV implements PrivValidator without any safety or persistence.
// Only use it for testing.
type MockPV struct {
	PrivateKeys          map[string]crypto.QuorumKeys
	// heightString -> quorumHash
	UpdateHeights        map[string]crypto.QuorumHash
	// quorumHash -> heightString
	FirstHeightOfQuorums map[string]string
	ProTxHash            crypto.ProTxHash
	mtx                  tmsync.RWMutex
	breakProposalSigning bool
	breakVoteSigning     bool
}

func NewMockPV() *MockPV {
	privKey := bls12381.GenPrivKey()
	quorumHash := crypto.RandQuorumHash()
	quorumKeys := crypto.QuorumKeys{
		PrivKey: privKey,
		PubKey: privKey.PubKey(),
		ThresholdPublicKey: privKey.PubKey(),
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &MockPV{PrivateKeys: privateKeysMap, UpdateHeights: nil, FirstHeightOfQuorums: nil,
		ProTxHash: crypto.RandProTxHash(), breakProposalSigning: false, breakVoteSigning: false}
}

func NewMockPVForQuorum(quorumHash crypto.QuorumHash) *MockPV {
	privKey := bls12381.GenPrivKey()
	quorumKeys := crypto.QuorumKeys{
		PrivKey: privKey,
		PubKey: privKey.PubKey(),
		ThresholdPublicKey: privKey.PubKey(),
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &MockPV{PrivateKeys: privateKeysMap, UpdateHeights: nil, FirstHeightOfQuorums: nil,
		ProTxHash: crypto.RandProTxHash(), breakProposalSigning: false, breakVoteSigning: false}
}

// NewMockPVWithParams allows one to create a MockPV instance, but with finer
// grained control over the operation of the mock validator. This is useful for
// mocking test failures.
func NewMockPVWithParams(privKey crypto.PrivKey, proTxHash crypto.ProTxHash, quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey, breakProposalSigning bool, breakVoteSigning bool) *MockPV {
	quorumKeys := crypto.QuorumKeys{
		PrivKey: privKey,
		PubKey: privKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &MockPV{PrivateKeys: privateKeysMap, UpdateHeights: nil, FirstHeightOfQuorums: nil,
		ProTxHash: proTxHash, breakProposalSigning: breakProposalSigning, breakVoteSigning: breakVoteSigning}
}

// GetPubKey implements PrivValidator.
func (pv *MockPV) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if keys, ok := pv.PrivateKeys[quorumHash.String()]; ok {
		return keys.PubKey, nil
	}
	return nil, fmt.Errorf("no public key for quorum hash %v", quorumHash)
}

// GetProTxHash implements PrivValidator.
func (pv *MockPV) GetProTxHash() (crypto.ProTxHash, error) {
	if len(pv.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("mock proTxHash is invalid size")
	}
	return pv.ProTxHash, nil
}

func (pv *MockPV) GetFirstQuorumHash() (crypto.QuorumHash, error) {
	for quorumHashString, _ := range pv.PrivateKeys {
		return hex.DecodeString(quorumHashString)
	}
	return nil, nil
}

// GetThresholdPublicKey ...
func (pv *MockPV) GetThresholdPublicKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	return pv.PrivateKeys[quorumHash.String()].ThresholdPublicKey, nil
}

// GetHeight ...
func (pv *MockPV) GetHeight(quorumHash crypto.QuorumHash) (int64, error) {
	intString := pv.FirstHeightOfQuorums[quorumHash.String()]
	return strconv.ParseInt(intString,10, 64)
}

// SignVote implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	useChainID := chainID
	if pv.breakVoteSigning {
		useChainID = "incorrect-chain-id"
	}

	blockSignId := VoteBlockSignId(useChainID, vote, quorumType, quorumHash)
	stateSignId := VoteStateSignId(useChainID, vote, quorumType, quorumHash)

	var privKey crypto.PrivKey
	if quorumKeys, ok := pv.PrivateKeys[quorumHash.String()]; ok {
		privKey = quorumKeys.PrivKey
	} else {
		return fmt.Errorf("file private validator could not sign vote for quorum hash %v", quorumHash)
	}

	blockSignature, err := privKey.SignDigest(blockSignId)
	// fmt.Printf("validator %X signing vote of type %d at height %d with key %X blockSignBytes %X stateSignBytes %X\n",
	//  pv.ProTxHash, vote.Type, vote.Height, pv.PrivKey.PubKey().Bytes(), blockSignBytes, stateSignBytes)
	// fmt.Printf("block sign bytes are %X by %X using key %X resulting in sig %X\n", blockSignBytes, pv.ProTxHash,
	//  pv.PrivKey.PubKey().Bytes(), blockSignature)
	if err != nil {
		return err
	}
	vote.BlockSignature = blockSignature

	if stateSignId != nil {
		stateSignature, err := privKey.SignDigest(stateSignId)
		if err != nil {
			return err
		}
		vote.StateSignature = stateSignature
	}

	return nil
}

// SignProposal Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) ([]byte, error) {
	useChainID := chainID
	if pv.breakProposalSigning {
		useChainID = "incorrect-chain-id"
	}

	signId := ProposalBlockSignId(useChainID, proposal, quorumType, quorumHash)

	var privKey crypto.PrivKey
	if quorumKeys, ok := pv.PrivateKeys[quorumHash.String()]; ok {
		privKey = quorumKeys.PrivKey
	} else {
		return signId, fmt.Errorf("file private validator could not sign vote for quorum hash %v", quorumHash)
	}

	sig, err := privKey.SignDigest(signId)
	if err != nil {
		return nil, err
	}

	proposal.Signature = sig

	return signId, nil
}

func (pv *MockPV) UpdatePrivateKey(privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, height int64) error {
	// fmt.Printf("mockpv node %X setting a new key %X at height %d\n", pv.ProTxHash,
	//  privateKey.PubKey().Bytes(), height)
	pv.mtx.RLock()
	pv.PrivateKeys[quorumHash.String()] = crypto.QuorumKeys{
		PrivKey: privateKey,
		PubKey: privateKey.PubKey(),
	}
	pv.UpdateHeights[strconv.Itoa(int(height))] = quorumHash
	if _, ok := pv.FirstHeightOfQuorums[quorumHash.String()]; ok != true {
		pv.FirstHeightOfQuorums[quorumHash.String()] = strconv.Itoa(int(height))
	}
	pv.mtx.RUnlock()
	return nil
}

func (pv *MockPV) ExtractIntoValidator(quorumHash crypto.QuorumHash) *Validator {
	pubKey, _ := pv.GetPubKey(quorumHash)
	if len(pv.ProTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &Validator{
		PubKey:      pubKey,
		VotingPower: DefaultDashVotingPower,
		ProTxHash:   pv.ProTxHash,
	}
}

// String returns a string representation of the MockPV.
func (pv *MockPV) String() string {
	proTxHash, _ := pv.GetProTxHash() // mockPV will never return an error, ignored here
	return fmt.Sprintf("MockPV{%v}", proTxHash)
}

// XXX: Implement.
func (pv *MockPV) DisableChecks() {
	// Currently this does nothing,
	// as MockPV has no safety checks at all.
}

type ErroringMockPV struct {
	MockPV
}

var ErroringMockPVErr = errors.New("erroringMockPV always returns an error")

// SignVote Implements PrivValidator.
func (pv *ErroringMockPV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	return ErroringMockPVErr
}

// SignProposal Implements PrivValidator.
func (pv *ErroringMockPV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) ([]byte, error) {
	return nil, ErroringMockPVErr
}

// NewErroringMockPV returns a MockPV that fails on each signing request. Again, for testing only.

func NewErroringMockPV() *ErroringMockPV {
	privKey := bls12381.GenPrivKey()
	quorumHash := crypto.RandQuorumHash()
	quorumKeys := crypto.QuorumKeys{
		PrivKey: privKey,
		PubKey: privKey.PubKey(),
		ThresholdPublicKey: privKey.PubKey(),
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &ErroringMockPV{MockPV{PrivateKeys: privateKeysMap, UpdateHeights: nil, FirstHeightOfQuorums: nil,
		ProTxHash: crypto.RandProTxHash(), breakProposalSigning: false, breakVoteSigning: false}}
}

type MockPrivValidatorsByProTxHash []*MockPV

func (pvs MockPrivValidatorsByProTxHash) Len() int {
	return len(pvs)
}

func (pvs MockPrivValidatorsByProTxHash) Less(i, j int) bool {
	pvi, err := pvs[i].GetProTxHash()
	if err != nil {
		panic(err)
	}
	pvj, err := pvs[j].GetProTxHash()
	if err != nil {
		panic(err)
	}

	return bytes.Compare(pvi, pvj) == -1
}

func (pvs MockPrivValidatorsByProTxHash) Swap(i, j int) {
	pvs[i], pvs[j] = pvs[j], pvs[i]
}

type GenesisValidatorsByProTxHash []GenesisValidator

func (vs GenesisValidatorsByProTxHash) Len() int {
	return len(vs)
}

func (vs GenesisValidatorsByProTxHash) Less(i, j int) bool {
	pvi := vs[i].ProTxHash
	pvj := vs[j].ProTxHash
	return bytes.Compare(pvi, pvj) == -1
}

func (vs GenesisValidatorsByProTxHash) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}
