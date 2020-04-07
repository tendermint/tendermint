package evidence

import (
	amino "github.com/tendermint/go-amino"

	cryptoamino "github.com/tendermint/tendermint/crypto/encoding/amino"
	ep "github.com/tendermint/tendermint/proto/evidence"
	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterMessages(cdc)
	cryptoamino.RegisterAmino(cdc)
	types.RegisterEvidences(cdc)
}

// For testing purposes only
func RegisterMockEvidences() {
	types.RegisterMockEvidences(cdc)
}

func MsgToProto(lm *ListMessage) (*ep.List, error) {
	evi := make([]tmproto.Evidence, len(lm.Evidence))
	for i := 0; i < len(lm.Evidence); i++ {
		le := lm.Evidence[i]
		ev, err := types.EvidenceToProto(le)
		if err != nil {
			return nil, err
		}
		evi[i] = *ev
	}

	epl := ep.List{
		Evidence: evi,
	}

	return &epl, nil
}

func MsgFromProto(pl ep.List) (*ListMessage, error) {
	evi := make([]types.Evidence, len(pl.Evidence))
	for i := 0; i < len(pl.Evidence); i++ {
		ev, err := types.EvidenceFromProto(pl.Evidence[i])
		if err != nil {
			return nil, err
		}
		evi[i] = ev
	}

	lm := ListMessage{
		Evidence: evi,
	}

	if err := lm.ValidateBasic(); err != nil {
		return nil, err
	}

	return &lm, nil
}
