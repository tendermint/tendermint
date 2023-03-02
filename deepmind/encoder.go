package deepmind

import (
	"fmt"

	pbcosmos "github.com/figment-networks/proto-cosmos/pb/sf/cosmos/type/v1"
	"github.com/golang/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/types"
)

func encodeBlock(bh types.EventDataNewBlock) ([]byte, error) {
	mappedEvidence, err := mapEvidence(&bh.Block.Evidence)
	if err != nil {
		return nil, err
	}

	mappedCommitSignatures, err := mapSignatures(bh.Block.LastCommit.Signatures)
	if err != nil {
		return nil, err
	}

	mappedResponseEndBlock, err := mapResponseEndBlock(&bh.ResultEndBlock)
	if err != nil {
		return nil, err
	}

	nb := &pbcosmos.Block{
		Header: &pbcosmos.Header{
			Version: &pbcosmos.Consensus{
				Block: bh.Block.Header.Version.Block,
				App:   bh.Block.Header.Version.App,
			},
			ChainId:            bh.Block.Header.ChainID,
			Height:             uint64(bh.Block.Header.Height),
			Time:               mapTimestamp(bh.Block.Header.Time),
			LastBlockId:        mapBlockID(bh.Block.LastBlockID),
			LastCommitHash:     bh.Block.Header.LastCommitHash,
			DataHash:           bh.Block.Header.DataHash,
			ValidatorsHash:     bh.Block.Header.ValidatorsHash,
			NextValidatorsHash: bh.Block.Header.NextValidatorsHash,
			ConsensusHash:      bh.Block.Header.ConsensusHash,
			AppHash:            bh.Block.Header.AppHash,
			LastResultsHash:    bh.Block.Header.LastResultsHash,
			EvidenceHash:       bh.Block.Header.EvidenceHash,
			ProposerAddress:    bh.Block.Header.ProposerAddress,
			Hash:               bh.Block.Header.Hash(),
		},
		Evidence: mappedEvidence,
		LastCommit: &pbcosmos.Commit{
			Height:     bh.Block.LastCommit.Height,
			Round:      bh.Block.LastCommit.Round,
			BlockId:    mapBlockID(bh.Block.LastCommit.BlockID),
			Signatures: mappedCommitSignatures,
		},
		ResultBeginBlock: mapResponseBeginBlock(&bh.ResultBeginBlock),
		ResultEndBlock:   mappedResponseEndBlock,
	}

	return proto.Marshal(nb)
}

func encodeTx(result *abci.TxResult) ([]byte, error) {
	mappedTx, err := mapTx(result.Tx)
	if err != nil {
		return nil, err
	}

	tx := &pbcosmos.TxResult{
		Hash:   tmhash.Sum(result.Tx),
		Height: uint64(result.Height),
		Index:  result.Index,
		Tx:     mappedTx,
		Result: &pbcosmos.ResponseDeliverTx{
			Code:      result.Result.Code,
			Data:      result.Result.Data,
			Log:       result.Result.Log,
			Info:      result.Result.Info,
			GasWanted: result.Result.GasWanted,
			GasUsed:   result.Result.GasUsed,
			Codespace: result.Result.Codespace,
		},
	}

	for _, ev := range result.Result.Events {
		tx.Result.Events = append(tx.Result.Events, mapEvent(ev))
	}

	return proto.Marshal(tx)
}

func encodeValidatorSetUpdates(updates *types.EventDataValidatorSetUpdates) ([]byte, error) {
	result := &pbcosmos.ValidatorSetUpdates{}

	for _, update := range updates.ValidatorUpdates {
		nPK := &pbcosmos.PublicKey{}

		switch update.PubKey.Type() {
		case "ed25519":
			nPK.Sum = &pbcosmos.PublicKey_Ed25519{Ed25519: update.PubKey.Bytes()}
		case "secp256k1":
			nPK.Sum = &pbcosmos.PublicKey_Secp256K1{Secp256K1: update.PubKey.Bytes()}
		default:
			return nil, fmt.Errorf("unsupported pubkey type: %T", update.PubKey)
		}

		result.ValidatorUpdates = append(result.ValidatorUpdates, &pbcosmos.Validator{
			Address:          update.Address.Bytes(),
			VotingPower:      update.VotingPower,
			ProposerPriority: update.ProposerPriority,
			PubKey:           nPK,
		})
	}

	return proto.Marshal(result)
}
