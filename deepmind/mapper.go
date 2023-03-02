package deepmind

import (
	"fmt"
	"time"

	pbcosmos "github.com/figment-networks/proto-cosmos/pb/sf/cosmos/type/v1"
	"github.com/golang/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	tmcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

func mapBlockID(bid types.BlockID) *pbcosmos.BlockID {
	return &pbcosmos.BlockID{
		Hash: bid.Hash,
		PartSetHeader: &pbcosmos.PartSetHeader{
			Total: bid.PartSetHeader.Total,
			Hash:  bid.PartSetHeader.Hash,
		},
	}
}

func mapEvidence(ed *types.EvidenceData) (*pbcosmos.EvidenceList, error) {
	result := &pbcosmos.EvidenceList{
		Evidence: []*pbcosmos.Evidence{},
	}

	for _, ev := range ed.Evidence {
		newEv := &pbcosmos.Evidence{}

		switch evN := ev.(type) {
		case *types.DuplicateVoteEvidence:
			newEv.Sum = &pbcosmos.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: &pbcosmos.DuplicateVoteEvidence{
					VoteA:            mapVote(evN.VoteA),
					VoteB:            mapVote(evN.VoteB),
					TotalVotingPower: evN.TotalVotingPower,
					ValidatorPower:   evN.ValidatorPower,
					Timestamp:        mapTimestamp(evN.Timestamp),
				},
			}
		case *types.LightClientAttackEvidence:
			mappedSetValidators, err := mapValidators(evN.ConflictingBlock.ValidatorSet.Validators)
			if err != nil {
				return nil, err
			}

			mappedByzantineValidators, err := mapValidators(evN.ByzantineValidators)
			if err != nil {
				return nil, err
			}

			mappedCommitSignatures, err := mapSignatures(evN.ConflictingBlock.Commit.Signatures)
			if err != nil {
				return nil, err
			}

			newEv.Sum = &pbcosmos.Evidence_LightClientAttackEvidence{
				LightClientAttackEvidence: &pbcosmos.LightClientAttackEvidence{
					ConflictingBlock: &pbcosmos.LightBlock{
						SignedHeader: &pbcosmos.SignedHeader{
							Header: &pbcosmos.Header{
								Version: &pbcosmos.Consensus{
									Block: evN.ConflictingBlock.Version.Block,
									App:   evN.ConflictingBlock.Version.App,
								},
								ChainId:            evN.ConflictingBlock.Header.ChainID,
								Height:             uint64(evN.ConflictingBlock.Header.Height),
								Time:               mapTimestamp(evN.ConflictingBlock.Header.Time),
								LastBlockId:        mapBlockID(evN.ConflictingBlock.Header.LastBlockID),
								LastCommitHash:     evN.ConflictingBlock.Header.LastCommitHash,
								DataHash:           evN.ConflictingBlock.Header.DataHash,
								ValidatorsHash:     evN.ConflictingBlock.Header.ValidatorsHash,
								NextValidatorsHash: evN.ConflictingBlock.Header.NextValidatorsHash,
								ConsensusHash:      evN.ConflictingBlock.Header.ConsensusHash,
								AppHash:            evN.ConflictingBlock.Header.AppHash,
								LastResultsHash:    evN.ConflictingBlock.Header.LastResultsHash,
								EvidenceHash:       evN.ConflictingBlock.Header.EvidenceHash,
								ProposerAddress:    evN.ConflictingBlock.Header.ProposerAddress,
							},
							Commit: &pbcosmos.Commit{
								Height:     evN.ConflictingBlock.Commit.Height,
								Round:      evN.ConflictingBlock.Commit.Round,
								BlockId:    mapBlockID(evN.ConflictingBlock.Commit.BlockID),
								Signatures: mappedCommitSignatures,
							},
						},
						ValidatorSet: &pbcosmos.ValidatorSet{
							Validators:       mappedSetValidators,
							Proposer:         mapProposer(evN.ConflictingBlock.ValidatorSet.Proposer),
							TotalVotingPower: evN.ConflictingBlock.ValidatorSet.TotalVotingPower(),
						},
					},
					CommonHeight:        evN.CommonHeight,
					ByzantineValidators: mappedByzantineValidators,
					TotalVotingPower:    evN.TotalVotingPower,
					Timestamp:           mapTimestamp(evN.Timestamp),
				},
			}

		default:
			return nil, fmt.Errorf("given type %T of EvidenceList mapping doesn't exist ", ev)
		}

		result.Evidence = append(result.Evidence, newEv)
	}

	return result, nil
}

func mapResponseBeginBlock(rbb *abci.ResponseBeginBlock) *pbcosmos.ResponseBeginBlock {
	result := &pbcosmos.ResponseBeginBlock{}

	for _, ev := range rbb.Events {
		result.Events = append(result.Events, mapEvent(ev))
	}

	return result
}

func mapResponseEndBlock(reb *abci.ResponseEndBlock) (*pbcosmos.ResponseEndBlock, error) {
	result := &pbcosmos.ResponseEndBlock{
		ConsensusParamUpdates: mapConsensusParams(reb.ConsensusParamUpdates),
	}

	for _, ev := range reb.Events {
		result.Events = append(result.Events, mapEvent(ev))
	}

	for _, vu := range reb.ValidatorUpdates {
		val, err := mapValidatorUpdate(vu)
		if err != nil {
			return nil, err
		}
		result.ValidatorUpdates = append(result.ValidatorUpdates, val)
	}

	return result, nil
}

func mapConsensusParams(cp *abci.ConsensusParams) *pbcosmos.ConsensusParams {
	result := &pbcosmos.ConsensusParams{}

	if cp.Block != nil {
		result.Block =
			&pbcosmos.BlockParams{
				MaxBytes: cp.Block.MaxBytes,
				MaxGas:   cp.Block.MaxGas,
			}
	}

	if cp.Evidence != nil {
		result.Evidence = &pbcosmos.EvidenceParams{
			MaxAgeNumBlocks: cp.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  mapDuration(cp.Evidence.MaxAgeDuration),
			MaxBytes:        cp.Evidence.MaxBytes,
		}
	}

	if cp.Validator != nil {
		result.Validator = &pbcosmos.ValidatorParams{
			PubKeyTypes: cp.Validator.PubKeyTypes,
		}
	}

	if cp.Version != nil {
		result.Version = &pbcosmos.VersionParams{
			AppVersion: cp.Version.AppVersion,
		}
	}

	return result
}

func mapProposer(val *types.Validator) *pbcosmos.Validator {
	nPK := &pbcosmos.PublicKey{}

	return &pbcosmos.Validator{
		Address:          val.Address,
		PubKey:           nPK,
		ProposerPriority: 0,
	}
}

func mapEvent(ev abci.Event) *pbcosmos.Event {
	cev := &pbcosmos.Event{EventType: ev.Type}

	for _, at := range ev.Attributes {
		cev.Attributes = append(cev.Attributes, &pbcosmos.EventAttribute{
			Key:   string(at.Key),
			Value: string(at.Value),
			Index: at.Index,
		})
	}

	return cev
}

func mapVote(edv *types.Vote) *pbcosmos.EventVote {
	return &pbcosmos.EventVote{
		EventVoteType:    pbcosmos.SignedMsgType(edv.Type),
		Height:           uint64(edv.Height),
		Round:            edv.Round,
		BlockId:          mapBlockID(edv.BlockID),
		Timestamp:        mapTimestamp(edv.Timestamp),
		ValidatorAddress: edv.ValidatorAddress,
		ValidatorIndex:   edv.ValidatorIndex,
		Signature:        edv.Signature,
	}
}

func mapSignatures(commitSignatures []types.CommitSig) ([]*pbcosmos.CommitSig, error) {
	signatures := make([]*pbcosmos.CommitSig, len(commitSignatures))
	for i, commitSignature := range commitSignatures {
		signature, err := mapSignature(commitSignature)
		if err != nil {
			return nil, err
		}
		signatures[i] = signature
	}
	return signatures, nil
}

func mapSignature(s types.CommitSig) (*pbcosmos.CommitSig, error) {
	return &pbcosmos.CommitSig{
		BlockIdFlag:      pbcosmos.BlockIDFlag(s.BlockIDFlag),
		ValidatorAddress: s.ValidatorAddress.Bytes(),
		Timestamp:        mapTimestamp(s.Timestamp),
		Signature:        s.Signature,
	}, nil
}

func mapValidatorUpdate(v abci.ValidatorUpdate) (*pbcosmos.ValidatorUpdate, error) {
	nPK := &pbcosmos.PublicKey{}
	var address []byte

	switch key := v.PubKey.Sum.(type) {
	case *crypto.PublicKey_Ed25519:
		nPK.Sum = &pbcosmos.PublicKey_Ed25519{Ed25519: key.Ed25519}
		address = tmcrypto.AddressHash(nPK.GetEd25519())
	case *crypto.PublicKey_Secp256K1:
		nPK.Sum = &pbcosmos.PublicKey_Secp256K1{Secp256K1: key.Secp256K1}
		address = tmcrypto.AddressHash(nPK.GetSecp256K1())
	default:
		return nil, fmt.Errorf("given type %T of PubKey mapping doesn't exist ", key)
	}

	return &pbcosmos.ValidatorUpdate{
		Address: address,
		PubKey:  nPK,
		Power:   v.Power,
	}, nil
}

func mapValidators(srcValidators []*types.Validator) ([]*pbcosmos.Validator, error) {
	validators := make([]*pbcosmos.Validator, len(srcValidators))
	for i, validator := range srcValidators {
		val, err := mapValidator(validator)
		if err != nil {
			return nil, err
		}
		validators[i] = val
	}
	return validators, nil
}

func mapValidator(v *types.Validator) (*pbcosmos.Validator, error) {
	nPK := &pbcosmos.PublicKey{}

	key := v.PubKey
	switch key.Type() {
	case ed25519.KeyType:
		nPK = &pbcosmos.PublicKey{
			Sum: &pbcosmos.PublicKey_Ed25519{Ed25519: key.Bytes()}}
	case secp256k1.KeyType:
		nPK = &pbcosmos.PublicKey{
			Sum: &pbcosmos.PublicKey_Secp256K1{Secp256K1: key.Bytes()}}
	default:
		return nil, fmt.Errorf("given type %T of PubKey mapping doesn't exist ", key)
	}

	// NOTE: See note in mapValidatorUpdate() about ProposerPriority

	return &pbcosmos.Validator{
		Address:          v.Address,
		PubKey:           nPK,
		VotingPower:      v.VotingPower,
		ProposerPriority: 0,
	}, nil
}

func mapTimestamp(time time.Time) *pbcosmos.Timestamp {
	return &pbcosmos.Timestamp{
		Seconds: time.Unix(),
		Nanos:   int32(time.UnixNano() - time.Unix()*1000000000),
	}
}

func mapDuration(d time.Duration) *pbcosmos.Duration {
	return &pbcosmos.Duration{
		Seconds: int64(d / time.Second),
		Nanos:   int32(d % time.Second),
	}
}

func mapTx(bytes []byte) (*pbcosmos.Tx, error) {
	t := &pbcosmos.Tx{}
	if err := proto.Unmarshal(bytes, t); err != nil {
		return nil, err
	}
	return t, nil
}
