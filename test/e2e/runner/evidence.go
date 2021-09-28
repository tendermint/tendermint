package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

// 1 in 4 evidence is light client evidence, the rest is duplicate vote evidence
const lightClientEvidenceRatio = 4

// InjectEvidence takes a running testnet and generates an amount of valid
// evidence and broadcasts it to a random node through the rpc endpoint `/broadcast_evidence`.
// Evidence is random and can be a mixture of LightClientAttackEvidence and
// DuplicateVoteEvidence.
func InjectEvidence(ctx context.Context, r *rand.Rand, testnet *e2e.Testnet, amount int) error {
	// select a random node
	var targetNode *e2e.Node

	for _, idx := range r.Perm(len(testnet.Nodes)) {
		targetNode = testnet.Nodes[idx]

		if targetNode.Mode == e2e.ModeSeed || targetNode.Mode == e2e.ModeLight {
			targetNode = nil
			continue
		}

		break
	}

	if targetNode == nil {
		return errors.New("could not find node to inject evidence into")
	}

	logger.Info(fmt.Sprintf("Injecting evidence through %v (amount: %d)...", targetNode.Name, amount))

	client, err := targetNode.Client()
	if err != nil {
		return err
	}

	// request the latest block and validator set from the node
	blockRes, err := client.Block(context.Background(), nil)
	if err != nil {
		return err
	}
	evidenceHeight := blockRes.Block.Height
	waitHeight := blockRes.Block.Height + 3

	nValidators := 100
	valRes, err := client.Validators(context.Background(), &evidenceHeight, nil, &nValidators)
	if err != nil {
		return err
	}

	valSet, err := types.ValidatorSetFromExistingValidators(valRes.Validators)
	if err != nil {
		return err
	}

	// get the private keys of all the validators in the network
	privVals, err := getPrivateValidatorKeys(testnet)
	if err != nil {
		return err
	}

	wctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	// wait for the node to reach the height above the forged height so that
	// it is able to validate the evidence
	_, err = waitForNode(wctx, targetNode, waitHeight)
	if err != nil {
		return err
	}

	var ev types.Evidence
	for i := 1; i <= amount; i++ {
		if i%lightClientEvidenceRatio == 0 {
			ev, err = generateLightClientAttackEvidence(
				privVals, evidenceHeight, valSet, testnet.Name, blockRes.Block.Time,
			)
		} else {
			ev, err = generateDuplicateVoteEvidence(
				privVals, evidenceHeight, valSet, testnet.Name, blockRes.Block.Time,
			)
		}
		if err != nil {
			return err
		}

		_, err := client.BroadcastEvidence(context.Background(), ev)
		if err != nil {
			return err
		}
	}

	wctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// wait for the node to reach the height above the forged height so that
	// it is able to validate the evidence
	_, err = waitForNode(wctx, targetNode, blockRes.Block.Height+2)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Finished sending evidence (height %d)", blockRes.Block.Height+2))

	return nil
}

func getPrivateValidatorKeys(testnet *e2e.Testnet) ([]types.MockPV, error) {
	privVals := []types.MockPV{}

	for _, node := range testnet.Nodes {
		if node.Mode == e2e.ModeValidator {
			privKeyPath := filepath.Join(testnet.Dir, node.Name, PrivvalKeyFile)
			privKey, err := readPrivKey(privKeyPath)
			if err != nil {
				return nil, err
			}
			// Create mock private validators from the validators private key. MockPV is
			// stateless which means we can double vote and do other funky stuff
			privVals = append(privVals, types.NewMockPVWithParams(privKey, false, false))
		}
	}

	return privVals, nil
}

// creates evidence of a lunatic attack. The height provided is the common height.
// The forged height happens 2 blocks later.
func generateLightClientAttackEvidence(
	privVals []types.MockPV,
	height int64,
	vals *types.ValidatorSet,
	chainID string,
	evTime time.Time,
) (*types.LightClientAttackEvidence, error) {
	// forge a random header
	forgedHeight := height + 2
	forgedTime := evTime.Add(1 * time.Second)
	header := makeHeaderRandom(chainID, forgedHeight)
	header.Time = forgedTime

	// add a new bogus validator and remove an existing one to
	// vary the validator set slightly
	pv, conflictingVals, err := mutateValidatorSet(privVals, vals)
	if err != nil {
		return nil, err
	}

	header.ValidatorsHash = conflictingVals.Hash()

	// create a commit for the forged header
	blockID := makeBlockID(header.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(chainID, forgedHeight, 0, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := factory.MakeCommit(blockID, forgedHeight, 0, voteSet, pv, forgedTime)
	if err != nil {
		return nil, err
	}

	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:     height,
		TotalVotingPower: vals.TotalVotingPower(),
		Timestamp:        evTime,
	}
	ev.ByzantineValidators = ev.GetByzantineValidators(vals, &types.SignedHeader{
		Header: makeHeaderRandom(chainID, forgedHeight),
	})
	return ev, nil
}

// generateDuplicateVoteEvidence picks a random validator from the val set and
// returns duplicate vote evidence against the validator
func generateDuplicateVoteEvidence(
	privVals []types.MockPV,
	height int64,
	vals *types.ValidatorSet,
	chainID string,
	time time.Time,
) (*types.DuplicateVoteEvidence, error) {
	privVal, valIdx, err := getRandomValidatorIndex(privVals, vals)
	if err != nil {
		return nil, err
	}
	voteA, err := factory.MakeVote(privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
	if err != nil {
		return nil, err
	}
	voteB, err := factory.MakeVote(privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
	if err != nil {
		return nil, err
	}
	ev, err := types.NewDuplicateVoteEvidence(voteA, voteB, time, vals)
	if err != nil {
		return nil, fmt.Errorf("could not generate evidence: %w", err)
	}

	return ev, nil
}

// getRandomValidatorIndex picks a random validator from a slice of mock PrivVals that's
// also part of the validator set, returning the PrivVal and its index in the validator set
func getRandomValidatorIndex(privVals []types.MockPV, vals *types.ValidatorSet) (types.MockPV, int32, error) {
	for _, idx := range rand.Perm(len(privVals)) {
		pv := privVals[idx]
		valIdx, _ := vals.GetByAddress(pv.PrivKey.PubKey().Address())
		if valIdx >= 0 {
			return pv, valIdx, nil
		}
	}
	return types.MockPV{}, -1, errors.New("no private validator found in validator set")
}

func readPrivKey(keyFilePath string) (crypto.PrivKey, error) {
	keyJSONBytes, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	pvKey := privval.FilePVKey{}
	err = tmjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		return nil, fmt.Errorf("error reading PrivValidator key from %v: %w", keyFilePath, err)
	}

	return pvKey.PrivKey, nil
}

func makeHeaderRandom(chainID string, height int64) *types.Header {
	return &types.Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               time.Now(),
		LastBlockID:        makeBlockID([]byte("headerhash"), 1000, []byte("partshash")),
		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
		DataHash:           crypto.CRandBytes(tmhash.Size),
		ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
		NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
		AppHash:            crypto.CRandBytes(tmhash.Size),
		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}

func makeRandomBlockID() types.BlockID {
	return makeBlockID(crypto.CRandBytes(tmhash.Size), 100, crypto.CRandBytes(tmhash.Size))
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}

func mutateValidatorSet(privVals []types.MockPV, vals *types.ValidatorSet,
) ([]types.PrivValidator, *types.ValidatorSet, error) {
	newVal, newPrivVal := factory.RandValidator(false, 10)

	var newVals *types.ValidatorSet
	if vals.Size() > 2 {
		newVals = types.NewValidatorSet(append(vals.Copy().Validators[:vals.Size()-1], newVal))
	} else {
		newVals = types.NewValidatorSet(append(vals.Copy().Validators, newVal))
	}

	// we need to sort the priv validators with the same index as the validator set
	pv := make([]types.PrivValidator, newVals.Size())
	for idx, val := range newVals.Validators {
		found := false
		for _, p := range append(privVals, newPrivVal.(types.MockPV)) {
			if bytes.Equal(p.PrivKey.PubKey().Address(), val.Address) {
				pv[idx] = p
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("missing priv validator for %v", val.Address)
		}
	}

	return pv, newVals, nil
}
