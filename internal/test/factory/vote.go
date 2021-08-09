package factory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func MakeVote(
	val consensus.PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step int,
	blockID meta.BlockID,
	time time.Time,
) (*consensus.Vote, error) {
	pubKey, err := val.GetPubKey(context.Background())
	if err != nil {
		return nil, err
	}
	v := &consensus.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(context.Background(), chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v, nil
}

func RandVoteSet(
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	numValidators int,
	votingPower int64,
) (*consensus.VoteSet, *consensus.ValidatorSet, []consensus.PrivValidator) {
	valSet, privValidators := RandValidatorPrivValSet(numValidators, votingPower)
	return consensus.NewVoteSet("test_chain_id", height, round, signedMsgType, valSet), valSet, privValidators
}

func RandValidatorPrivValSet(numValidators int, votingPower int64) (*consensus.ValidatorSet, []consensus.PrivValidator) {
	var (
		valz           = make([]*consensus.Validator, numValidators)
		privValidators = make([]consensus.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(consensus.PrivValidatorsByAddress(privValidators))

	return consensus.NewValidatorSet(valz), privValidators
}

func DeterministicVoteSet(
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	votingPower int64,
) (*consensus.VoteSet, *consensus.ValidatorSet, []consensus.PrivValidator) {
	valSet, privValidators := DeterministicValidatorSet()
	return consensus.NewVoteSet("test_chain_id", height, round, signedMsgType, valSet), valSet, privValidators
}

func DeterministicValidatorSet() (*consensus.ValidatorSet, []consensus.PrivValidator) {
	var (
		valz           = make([]*consensus.Validator, 10)
		privValidators = make([]consensus.PrivValidator, 10)
	)

	for i := 0; i < 10; i++ {
		// val, privValidator := DeterministicValidator(ed25519.PrivKey([]byte(deterministicKeys[i])))
		val, privValidator := DeterministicValidator(ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("key: %x", i))))
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(consensus.PrivValidatorsByAddress(privValidators))

	return consensus.NewValidatorSet(valz), privValidators
}

func DeterministicValidator(key crypto.PrivKey) (*consensus.Validator, consensus.PrivValidator) {
	privVal := consensus.NewMockPV()
	privVal.PrivKey = key
	var votePower int64 = 50
	pubKey, err := privVal.GetPubKey(context.TODO())
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := consensus.NewValidator(pubKey, votePower)
	return val, privVal
}
