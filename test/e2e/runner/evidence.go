package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
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
func InjectEvidence(ctx context.Context, logger log.Logger, r *rand.Rand, testnet *e2e.Testnet, amount int) error {
	// select a random node
	var targetNode *e2e.Node

	for _, idx := range r.Perm(len(testnet.Nodes)) {
		if !testnet.Nodes[idx].Stateless() {
			targetNode = testnet.Nodes[idx]
			break
		}
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
	blockRes, err := client.Block(ctx, nil)
	if err != nil {
		return err
	}
	evidenceHeight := blockRes.Block.Height - 3

	nValidators := 100
	valRes, err := client.Validators(ctx, &evidenceHeight, nil, &nValidators)
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

	// request the latest block and validator set from the node
	blockRes, err = client.Block(ctx, &evidenceHeight)
	if err != nil {
		return err
	}

	var ev types.Evidence
	for i := 1; i <= amount; i++ {
		if i%lightClientEvidenceRatio == 0 {
			ev, err = generateLightClientAttackEvidence(ctx,
				privVals, evidenceHeight, valSet, testnet.Name, blockRes.Block.Time,
			)
		} else {
			var dve *types.DuplicateVoteEvidence
			dve, err = generateDuplicateVoteEvidence(ctx,
				privVals, evidenceHeight, valSet, testnet.Name, blockRes.Block.Time,
			)
			if dve.VoteA.Height < testnet.VoteExtensionsEnableHeight {
				dve.VoteA.StripExtension()
				dve.VoteB.StripExtension()
			}
			ev = dve
		}
		if err != nil {
			return err
		}

		_, err := client.BroadcastEvidence(ctx, ev)
		if err != nil {
			return err
		}
	}

	logger.Info("Finished sending evidence",
		"node", testnet.Name,
		"amount", amount,
		"height", evidenceHeight,
	)

	wctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	// wait for the node to make progress after submitting
	// evidence (3 (forged height) + 1 (progress))
	_, err = waitForNode(wctx, logger, targetNode, evidenceHeight+4)
	if err != nil {
		return err
	}

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
	ctx context.Context,
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
	pv, conflictingVals, err := mutateValidatorSet(ctx, privVals, vals)
	if err != nil {
		return nil, err
	}

	header.ValidatorsHash = conflictingVals.Hash()

	// create a commit for the forged header
	blockID := makeBlockID(header.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewExtendedVoteSet(chainID, forgedHeight, 0, tmproto.SignedMsgType(2), conflictingVals)

	ec, err := factory.MakeExtendedCommit(ctx, blockID, forgedHeight, 0, voteSet, pv, forgedTime)
	if err != nil {
		return nil, err
	}

	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: header,
				Commit: ec.ToCommit(),
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
	ctx context.Context,
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

	voteA, err := factory.MakeVote(ctx, privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
	if err != nil {
		return nil, err
	}
	voteB, err := factory.MakeVote(ctx, privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
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
	keyJSONBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	pvKey := privval.FilePVKey{}
	err = json.Unmarshal(keyJSONBytes, &pvKey)
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
		LastCommitHash:     crypto.CRandBytes(crypto.HashSize),
		DataHash:           crypto.CRandBytes(crypto.HashSize),
		ValidatorsHash:     crypto.CRandBytes(crypto.HashSize),
		NextValidatorsHash: crypto.CRandBytes(crypto.HashSize),
		ConsensusHash:      crypto.CRandBytes(crypto.HashSize),
		AppHash:            crypto.CRandBytes(crypto.HashSize),
		LastResultsHash:    crypto.CRandBytes(crypto.HashSize),
		EvidenceHash:       crypto.CRandBytes(crypto.HashSize),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}

func makeRandomBlockID() types.BlockID {
	return makeBlockID(crypto.CRandBytes(crypto.HashSize), 100, crypto.CRandBytes(crypto.HashSize))
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, crypto.HashSize)
		psH = make([]byte, crypto.HashSize)
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

func mutateValidatorSet(ctx context.Context, privVals []types.MockPV, vals *types.ValidatorSet) ([]types.PrivValidator, *types.ValidatorSet, error) {
	newVal, newPrivVal, err := factory.Validator(ctx, 10)
	if err != nil {
		return nil, nil, err
	}

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
