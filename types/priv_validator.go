package types

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// PrivValidatorType defines the implementation types.
type PrivValidatorType uint8

const (
	MockSignerClient      = PrivValidatorType(0x00) // mock signer
	FileSignerClient      = PrivValidatorType(0x01) // signer client via file
	RetrySignerClient     = PrivValidatorType(0x02) // signer client with retry via socket
	SignerSocketClient    = PrivValidatorType(0x03) // signer client via socket
	ErrorMockSignerClient = PrivValidatorType(0x04) // error mock signer
	SignerGRPCClient      = PrivValidatorType(0x05) // signer client via gRPC
	DashCoreRPCClient     = PrivValidatorType(0x06) // signer client via gRPC
)

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes and proposals, and never double signs.
type PrivValidator interface {
	GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error)
	UpdatePrivateKey(
		ctx context.Context,
		privateKey crypto.PrivKey,
		quorumHash crypto.QuorumHash,
		thresholdPublicKey crypto.PubKey,
		height int64,
	)

	GetProTxHash(context.Context) (crypto.ProTxHash, error)
	GetFirstQuorumHash(context.Context) (crypto.QuorumHash, error)
	GetPrivateKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error)
	GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error)
	GetHeight(ctx context.Context, quorumHash crypto.QuorumHash) (int64, error)

	SignVote(
		ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
		vote *tmproto.Vote, stateID StateID, logger log.Logger) error
	SignProposal(
		ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
		proposal *tmproto.Proposal) ([]byte, error)

	ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *Validator
}

type PrivValidatorsByProTxHash []PrivValidator

func (pvs PrivValidatorsByProTxHash) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByProTxHash) Less(i, j int) bool {
	pvi, err := pvs[i].GetProTxHash(context.Background())
	if err != nil {
		panic(err)
	}
	pvj, err := pvs[j].GetProTxHash(context.Background())
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
	PrivateKeys map[string]crypto.QuorumKeys
	// heightString -> quorumHash
	UpdateHeights map[string]crypto.QuorumHash
	// quorumHash -> heightString
	FirstHeightOfQuorums map[string]string
	ProTxHash            crypto.ProTxHash
	mtx                  sync.RWMutex
	breakProposalSigning bool
	breakVoteSigning     bool
}

// GenKeysForQuorumHash generates the quorum keys for passed quorum hash
func GenKeysForQuorumHash(quorumHash crypto.QuorumHash) func(pv *MockPV) {
	return func(pv *MockPV) {
		pv.PrivateKeys[quorumHash.String()] = generateQuorumKeys()
	}
}

// UseProTxHash sets pro-tx-hash to a private-validator
func UseProTxHash(proTxHash ProTxHash) func(pv *MockPV) {
	return func(pv *MockPV) {
		pv.ProTxHash = proTxHash
	}
}

func generateQuorumKeys() crypto.QuorumKeys {
	privKey := bls12381.GenPrivKey()
	return crypto.QuorumKeys{
		PrivKey:            privKey,
		PubKey:             privKey.PubKey(),
		ThresholdPublicKey: privKey.PubKey(),
	}
}

func NewMockPV(opts ...func(pv *MockPV)) *MockPV {
	pv := &MockPV{
		PrivateKeys:          make(map[string]crypto.QuorumKeys),
		UpdateHeights:        make(map[string]crypto.QuorumHash),
		FirstHeightOfQuorums: make(map[string]string),
		ProTxHash:            crypto.RandProTxHash(),
		breakProposalSigning: false,
		breakVoteSigning:     false,
	}
	for _, opt := range opts {
		opt(pv)
	}
	if len(pv.PrivateKeys) == 0 {
		quorumHash := crypto.RandQuorumHash()
		pv.PrivateKeys[quorumHash.String()] = generateQuorumKeys()
	}
	return pv
}

func NewMockPVForQuorum(quorumHash crypto.QuorumHash) *MockPV {
	return NewMockPV(GenKeysForQuorumHash(quorumHash))
}

// NewMockPVWithParams allows one to create a MockPV instance, but with finer
// grained control over the operation of the mock validator. This is useful for
// mocking test failures.
func NewMockPVWithParams(
	privKey crypto.PrivKey,
	proTxHash crypto.ProTxHash,
	quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey,
	breakProposalSigning bool,
	breakVoteSigning bool,
) *MockPV {
	quorumKeys := crypto.QuorumKeys{
		PrivKey:            privKey,
		PubKey:             privKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	updateHeightsMap := make(map[string]crypto.QuorumHash)
	firstHeightOfQuorumsMap := make(map[string]string)

	return &MockPV{
		PrivateKeys:          privateKeysMap,
		UpdateHeights:        updateHeightsMap,
		FirstHeightOfQuorums: firstHeightOfQuorumsMap,
		ProTxHash:            proTxHash,
		breakProposalSigning: breakProposalSigning,
		breakVoteSigning:     breakVoteSigning,
	}
}

// GetPubKey implements PrivValidator.
func (pv *MockPV) GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	if keys, ok := pv.PrivateKeys[quorumHash.String()]; ok {
		return keys.PubKey, nil
	}
	return nil, fmt.Errorf("mockPV: no public key for quorum hash %v", quorumHash)
}

// GetProTxHash implements PrivValidator.
func (pv *MockPV) GetProTxHash(ctx context.Context) (crypto.ProTxHash, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if len(pv.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("mock proTxHash is invalid size")
	}
	return pv.ProTxHash, nil
}

func (pv *MockPV) GetFirstQuorumHash(ctx context.Context) (crypto.QuorumHash, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	for quorumHashString := range pv.PrivateKeys {
		return hex.DecodeString(quorumHashString)
	}
	return nil, nil
}

// GetThresholdPublicKey ...
func (pv *MockPV) GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	return pv.PrivateKeys[quorumHash.String()].ThresholdPublicKey, nil
}

// GetPrivateKey ...
func (pv *MockPV) GetPrivateKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	return pv.PrivateKeys[quorumHash.String()].PrivKey, nil
}

func (pv *MockPV) getPrivateKey(quorumHash crypto.QuorumHash) crypto.PrivKey {
	return pv.PrivateKeys[quorumHash.String()].PrivKey
}

// ThresholdPublicKeyForQuorumHash ...
func (pv *MockPV) ThresholdPublicKeyForQuorumHash(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	return pv.PrivateKeys[quorumHash.String()].ThresholdPublicKey, nil
}

// GetHeight ...
func (pv *MockPV) GetHeight(ctx context.Context, quorumHash crypto.QuorumHash) (int64, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()
	if intString, ok := pv.FirstHeightOfQuorums[quorumHash.String()]; ok {
		return strconv.ParseInt(intString, 10, 64)
	}
	return -1, fmt.Errorf("quorum hash not found for GetHeight %v", quorumHash.String())
}

// SignVote implements PrivValidator.
func (pv *MockPV) SignVote(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vote *tmproto.Vote,
	stateID StateID,
	logger log.Logger) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	useChainID := chainID
	if pv.breakVoteSigning {
		useChainID = "incorrect-chain-id"
	}

	blockSignID := VoteBlockSignID(useChainID, vote, quorumType, quorumHash)

	privKey := pv.getPrivateKey(quorumHash)

	blockSignature, err := privKey.SignDigest(blockSignID)
	// fmt.Printf("validator %X signing vote of type %d at height %d with key %X blockSignBytes %X stateSignBytes %X\n",
	//  pv.ProTxHash, vote.Type, vote.Height, pv.PrivKey.PubKey().Bytes(), blockSignBytes, stateSignBytes)
	// fmt.Printf("block sign bytes are %X by %X using key %X resulting in sig %X\n", blockSignBytes, pv.ProTxHash,
	//  pv.PrivKey.PubKey().Bytes(), blockSignature)
	if err != nil {
		return err
	}
	vote.BlockSignature = blockSignature

	if vote.BlockID.Hash != nil {
		stateSignID := stateID.SignID(useChainID, quorumType, quorumHash)
		stateSignature, err := privKey.SignDigest(stateSignID)
		if err != nil {
			return err
		}
		vote.StateSignature = stateSignature
	}

	var extSigs [][]byte
	// We only sign vote extensions for precommits
	if vote.Type == tmproto.PrecommitType {
		extSignIDs, err := MakeVoteExtensionSignIDs(useChainID, vote, quorumType, quorumHash)
		if err != nil {
			return err
		}
		for _, extSignID := range extSignIDs {
			extSig, err := privKey.SignDigest(extSignID.ID)
			if err != nil {
				return err
			}
			extSigs = append(extSigs, extSig)
		}
	} else if len(vote.VoteExtensions) > 0 {
		return errors.New("unexpected vote extension - vote extensions are only allowed in precommits")
	}
	if len(extSigs) > 0 {
		for i := range vote.VoteExtensions {
			vote.VoteExtensions[i].Signature = extSigs[i]
		}
	}
	return nil
}

// SignProposal Implements PrivValidator.
func (pv *MockPV) SignProposal(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	proposal *tmproto.Proposal,
) ([]byte, error) {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if pv.breakProposalSigning {
		chainID = "incorrect-chain-id"
	}

	signID := ProposalBlockSignID(chainID, proposal, quorumType, quorumHash)

	quorumKeys, ok := pv.PrivateKeys[quorumHash.String()]
	if !ok {
		return signID, fmt.Errorf("file private validator could not sign vote for quorum hash %v", quorumHash)
	}

	sig, err := quorumKeys.PrivKey.SignDigest(signID)
	if err != nil {
		return nil, err
	}

	proposal.Signature = sig

	return signID, nil
}

func (pv *MockPV) UpdatePrivateKey(
	ctx context.Context,
	privateKey crypto.PrivKey,
	quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey,
	height int64,
) {
	// fmt.Printf("mockpv node %X setting a new key %X at height %d\n", pv.ProTxHash,
	//  privateKey.PubKey().Bytes(), height)
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	pv.PrivateKeys[quorumHash.String()] = crypto.QuorumKeys{
		PrivKey:            privateKey,
		PubKey:             privateKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	pv.UpdateHeights[strconv.Itoa(int(height))] = quorumHash
	if _, ok := pv.FirstHeightOfQuorums[quorumHash.String()]; !ok {
		pv.FirstHeightOfQuorums[quorumHash.String()] = strconv.Itoa(int(height))
	}
}

func (pv *MockPV) ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *Validator {
	pubKey, _ := pv.GetPubKey(ctx, quorumHash)
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
	proTxHash, _ := pv.GetProTxHash(context.TODO()) // mockPV will never return an error, ignored here
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

// GetPubKey Implements PrivValidator.
func (pv *ErroringMockPV) GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	return nil, ErroringMockPVErr
}

// SignVote Implements PrivValidator.
func (pv *ErroringMockPV) SignVote(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
	vote *tmproto.Vote, stateID StateID, logger log.Logger) error {
	return ErroringMockPVErr
}

// SignProposal Implements PrivValidator.
func (pv *ErroringMockPV) SignProposal(
	ctx context.Context, chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	proposal *tmproto.Proposal,
) ([]byte, error) {
	return nil, ErroringMockPVErr
}

// NewErroringMockPV returns a MockPV that fails on each signing request. Again, for testing only.

func NewErroringMockPV() *ErroringMockPV {
	privKey := bls12381.GenPrivKey()
	quorumHash := crypto.RandQuorumHash()
	quorumKeys := crypto.QuorumKeys{
		PrivKey:            privKey,
		PubKey:             privKey.PubKey(),
		ThresholdPublicKey: privKey.PubKey(),
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &ErroringMockPV{
		MockPV{
			PrivateKeys:          privateKeysMap,
			UpdateHeights:        nil,
			FirstHeightOfQuorums: nil,
			ProTxHash:            crypto.RandProTxHash(),
			breakProposalSigning: false,
			breakVoteSigning:     false,
		},
	}
}

type MockPrivValidatorsByProTxHash []*MockPV

func (pvs MockPrivValidatorsByProTxHash) Len() int {
	return len(pvs)
}

func (pvs MockPrivValidatorsByProTxHash) Less(i, j int) bool {
	pvi := pvs[i].ProTxHash
	pvj := pvs[j].ProTxHash

	return bytes.Compare(pvi, pvj) == -1
}

func (pvs MockPrivValidatorsByProTxHash) Swap(i, j int) {
	pvs[i], pvs[j] = pvs[j], pvs[i]
}
