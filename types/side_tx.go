package types

import (
	"encoding/binary"
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// SideTxResult side tx result for vote
type SideTxResult struct {
	TxHash []byte `json:"tx_hash"`
	Result int32  `json:"result"`
	Sig    []byte `json:"sig"`
}

func (sp *SideTxResult) String() string {
	if sp == nil {
		return ""
	}

	return fmt.Sprintf("SideTxResult{%X (%v) %X}",
		cmn.Fingerprint(sp.TxHash),
		sp.Result,
		cmn.Fingerprint(sp.Sig),
	)
}

// SideTxResultWithData side tx result with data for vote
type SideTxResultWithData struct {
	SideTxResult

	Data []byte `json:"data"`
}

// GetBytes returns data bytes for sign
func (sp *SideTxResultWithData) GetBytes() []byte {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(sp.Result))

	data := make([]byte, 0)
	data = append(data, bs[3]) // use last byte as result
	if len(sp.Data) > 0 {
		data = append(data, sp.Data...)
	}
	return data
}

func (sp *SideTxResultWithData) String() string {
	if sp == nil {
		return ""
	}

	return fmt.Sprintf("SideTxResultWithData {%s %X}",
		sp.SideTxResult.String(),
		cmn.Fingerprint(sp.Data),
	)
}
