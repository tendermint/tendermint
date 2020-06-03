package evidence

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	ep "github.com/tendermint/tendermint/proto/evidence"
	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/types"
)

// encodemsg takes a array of evidence
// returns the byte encoding of the List Message
func encodeMsg(evis []types.Evidence) ([]byte, error) {
	evi := make([]tmproto.Evidence, len(evis))
	for i := 0; i < len(evis); i++ {
		le := evis[i]
		ev, err := types.EvidenceToProto(le)
		if err != nil {
			return nil, err
		}
		evi[i] = *ev
	}

	epl := ep.List{
		Evidence: evi,
	}

	return proto.Marshal(&epl)
}

// decodemsg takes an array of bytes
// returns an array of evidence
func decodeMsg(bz []byte) (evis []types.Evidence, err error) {
	lm := ep.List{}
	proto.Unmarshal(bz, &lm)

	evis = make([]types.Evidence, len(lm.Evidence))
	for i := 0; i < len(lm.Evidence); i++ {
		ev, err := types.EvidenceFromProto(&lm.Evidence[i])
		if err != nil {
			return nil, err
		}
		evis[i] = ev
	}

	for i, ev := range evis {
		if err := ev.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}

	return evis, nil
}
