package types

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"

	tmsync "github.com/tendermint/tendermint/libs/sync"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes and proposals, and never double signs.
type PrivValidator interface {
	GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error)
	UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error

	GetProTxHash() (crypto.ProTxHash, error)

	SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error
	SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error

	ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *Validator
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
	PrivKey              crypto.PrivKey
	NextPrivKeys         []crypto.PrivKey
	NextPrivKeyHeights   []int64
	ProTxHash            crypto.ProTxHash
	mtx                  tmsync.RWMutex
	breakProposalSigning bool
	breakVoteSigning     bool
}

func NewMockPV() *MockPV {
	return &MockPV{PrivKey: bls12381.GenPrivKey(), NextPrivKeys: nil, NextPrivKeyHeights: nil,
		ProTxHash: crypto.RandProTxHash(), breakProposalSigning: false, breakVoteSigning: false}
}

// NewMockPVWithParams allows one to create a MockPV instance, but with finer
// grained control over the operation of the mock validator. This is useful for
// mocking test failures.
func NewMockPVWithParams(privKey crypto.PrivKey, proTxHash []byte, breakProposalSigning,
	breakVoteSigning bool) *MockPV {
	return &MockPV{PrivKey: privKey, NextPrivKeys: nil, NextPrivKeyHeights: nil, ProTxHash: proTxHash,
		breakProposalSigning: breakProposalSigning, breakVoteSigning: breakVoteSigning}
}

// Implements PrivValidator.
func (pv *MockPV) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	return pv.PrivKey.PubKey(), nil
}

// Implements PrivValidator.
func (pv *MockPV) GetProTxHash() (crypto.ProTxHash, error) {
	if len(pv.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("mock proTxHash is invalid size")
	}
	return pv.ProTxHash, nil
}

// Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	pv.updateKeyIfNeeded(vote.Height)
	useChainID := chainID
	if pv.breakVoteSigning {
		useChainID = "incorrect-chain-id"
	}

	blockSignId := VoteBlockSignId(useChainID, vote, quorumType, quorumHash)
	stateSignId := VoteStateSignId(useChainID, vote, quorumType, quorumHash)

	blockSignature, err := pv.PrivKey.SignDigest(blockSignId)
	// fmt.Printf("validator %X signing vote of type %d at height %d with key %X blockSignBytes %X stateSignBytes %X\n",
	//  pv.ProTxHash, vote.Type, vote.Height, pv.PrivKey.PubKey().Bytes(), blockSignBytes, stateSignBytes)
	// fmt.Printf("block sign bytes are %X by %X using key %X resulting in sig %X\n", blockSignBytes, pv.ProTxHash,
	//  pv.PrivKey.PubKey().Bytes(), blockSignature)
	if err != nil {
		return err
	}
	vote.BlockSignature = blockSignature

	if stateSignId != nil {
		stateSignature, err := pv.PrivKey.SignDigest(stateSignId)
		if err != nil {
			return err
		}
		vote.StateSignature = stateSignature
	}

	return nil
}

// Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	pv.updateKeyIfNeeded(proposal.Height)
	useChainID := chainID
	if pv.breakProposalSigning {
		useChainID = "incorrect-chain-id"
	}

	signId := ProposalBlockSignId(useChainID, proposal, quorumType, quorumHash)

	fmt.Printf("mock proposer %X \nsigning proposal at height %d \nwith key %X \nquorumType %d \nquorumHash %X\n proposalSignId %X\n", pv.ProTxHash,
	 proposal.Height, pv.PrivKey.PubKey().Bytes(), quorumType, quorumHash, signId)
	sig, err := pv.PrivKey.SignDigest(signId)
	if err != nil {
		return err
	}

	proposal.Signature = sig

	return nil
}

func (pv *MockPV) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// fmt.Printf("mockpv node %X setting a new key %X at height %d\n", pv.ProTxHash,
	//  privateKey.PubKey().Bytes(), height)
	pv.mtx.RLock()
	pv.NextPrivKeys = append(pv.NextPrivKeys, privateKey)
	pv.NextPrivKeyHeights = append(pv.NextPrivKeyHeights, height)
	pv.mtx.RUnlock()
	return nil
}

func (pv *MockPV) updateKeyIfNeeded(height int64) {
	pv.mtx.RLock()
	if pv.NextPrivKeys != nil && len(pv.NextPrivKeys) > 0 && pv.NextPrivKeyHeights != nil &&
		len(pv.NextPrivKeyHeights) > 0 && height >= pv.NextPrivKeyHeights[0] {
		// fmt.Printf("mockpv node %X at height %d updating key %X with new key %X\n", pv.ProTxHash,
		// height, pv.PrivKey.PubKey().Bytes(), pv.NextPrivKeys[0].PubKey().Bytes())
		pv.PrivKey = pv.NextPrivKeys[0]
		if len(pv.NextPrivKeys) > 1 {
			pv.NextPrivKeys = pv.NextPrivKeys[1:]
			pv.NextPrivKeyHeights = pv.NextPrivKeyHeights[1:]
		} else {
			pv.NextPrivKeys = nil
			pv.NextPrivKeyHeights = nil
		}
	}
	// else {
	//	fmt.Printf("mockpv node %X at height %d did not update key %X with next keys %v\n", pv.ProTxHash,
	//  	height, pv.PrivKey.PubKey().Bytes(), pv.NextPrivKeyHeights)
	// }
	pv.mtx.RUnlock()
}

func (pv *MockPV) ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *Validator {
	var pubKey crypto.PubKey
	if pv.NextPrivKeys != nil && len(pv.NextPrivKeys) > 0 && height >= pv.NextPrivKeyHeights[0] {
		for i, nextPrivKeyHeight := range pv.NextPrivKeyHeights {
			if height >= nextPrivKeyHeight {
				pubKey = pv.NextPrivKeys[i].PubKey()
			}
		}
	} else {
		pubKey, _ = pv.GetPubKey(quorumHash)
	}
	if len(pv.ProTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: DefaultDashVotingPower,
		ProTxHash:   pv.ProTxHash,
	}
}

// String returns a string representation of the MockPV.
func (pv *MockPV) String() string {
	mpv, _ := pv.GetPubKey([]byte{}) // mockPV will never return an error, ignored here
	return fmt.Sprintf("MockPV{%v}", mpv.Address())
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

// Implements PrivValidator.
func (pv *ErroringMockPV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	return ErroringMockPVErr
}

// Implements PrivValidator.
func (pv *ErroringMockPV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	return ErroringMockPVErr
}

// NewErroringMockPV returns a MockPV that fails on each signing request. Again, for testing only.

func NewErroringMockPV() *ErroringMockPV {
	return &ErroringMockPV{MockPV{PrivKey: bls12381.GenPrivKey(), NextPrivKeys: nil, NextPrivKeyHeights: nil,
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
