package types

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// SideTxResult side tx result for vote
type SideTxResult struct {
	TxHash []byte `json:"tx_hash"`
	Result byte   `json:"result"`
	Sig    []byte `json:"sig"`
}

func (sp *SideTxResult) String() string {
	if sp == nil {
		return ""
	}

	return fmt.Sprintf("SideTxResult{%X (Result: %v) %X}",
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
	data := make([]byte, 0)
	data = append(data, sp.Result)
	data = append(data, sp.Data...)
	return data
}
