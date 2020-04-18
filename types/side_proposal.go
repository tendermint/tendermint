package types

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// SideProposalResult side proposal result for vote
type SideProposalResult struct {
	TxHash []byte `json:"tx_hash"`
	Result byte   `json:"result"`
	Sig    []byte `json:"sig"`
}

func (sp *SideProposalResult) String() string {
	if sp == nil {
		return ""
	}

	return fmt.Sprintf("SideProposalResult{%X (Result: %v) %X}",
		cmn.Fingerprint(sp.TxHash),
		sp.Result,
		cmn.Fingerprint(sp.Sig),
	)
}
