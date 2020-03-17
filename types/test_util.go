package types

import (
	"time"

	"github.com/pkg/errors"
)

func MakeCommit(blockID BlockID, height int64, round int32,
	voteSet *VoteSet, validators []PrivValidator, now time.Time) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey()
		if err != nil {
			return nil, errors.Wrap(err, "can't get pubkey")
		}
		vote := &Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   uint32(i),
			Height:           height,
			Round:            round,
			Type:             PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	err = privVal.SignVote(voteSet.ChainID(), vote)
	if err != nil {
		return false, err
	}
	return voteSet.AddVote(vote)
}

func MakeVote(
	height int64,
	blockID BlockID,
	valSet *ValidatorSet,
	privVal PrivValidator,
	chainID string,
	now time.Time,
) (*Vote, error) {
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		return nil, errors.Wrap(err, "can't get pubkey")
	}
	addr := pubKey.Address()
	idx, _ := valSet.GetByAddress(addr)
	vote := &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            0,
		Timestamp:        now,
		Type:             PrecommitType,
		BlockID:          blockID,
	}
	if err := privVal.SignVote(chainID, vote); err != nil {
		return nil, err
	}
	return vote, nil
}

// MakeBlock returns a new block with an empty header, except what can be
// computed from itself.
// It populates the same set of fields validated by ValidateBasic.
func MakeBlock(height int64, txs Txs, lastCommit *Commit, evidence []EvidenceI) *Block {
	// Handle going from the evidence Inteface to Evidence type (protobuf)
	evi := make([]Evidence, len(evidence))
	for i := range evidence {
		evi[i].SetEvidenceI(evidence[i])
	}
	block := &Block{
		TmBlock: TmBlock{
			Header: Header{
				Height: height,
			},
			Data: Data{
				Txs: txs.Bytes(),
			},
			Evidence:   EvidenceData{Evidence: evi},
			LastCommit: lastCommit,
		},
	}
	block.fillHeader()
	return block
}
